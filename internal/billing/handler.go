package billing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"server/internal/events"
	"server/internal/htmlutil"
	"server/internal/session"
	"server/internal/user"
	"strconv"
	"strings"
	"time"
)

const (
	SignatureHeader = "Paddle-Signature"
	maxWebhookBytes = 1 << 20
)

const (
	PlanMonthly = "monthly"
	PlanYearly  = "yearly"
)

const MsgPortalDown = "Subscription management is unavailable right now. Try again in a few minutes"

var PayPolicy = strings.Join([]string{
	"default-src 'self'",
	"base-uri 'none'",
	"object-src 'none'",
	"frame-ancestors 'none'",
	"form-action 'self'",
	"script-src 'self' 'unsafe-inline' https://cdn.paddle.com",
	"style-src 'self' 'unsafe-inline' https://cdn.paddle.com https://sandbox-cdn.paddle.com",
	"frame-src https://*.paddle.com",
	"connect-src 'self' https://*.paddle.com",
	"img-src 'self' data: https://*.paddle.com",
	"font-src 'self' data: https://*.paddle.com",
}, "; ")

type subscriptions interface {
	Apply(ctx context.Context, e Event, access time.Time) (Outcome, int64, error)
	Find(ctx context.Context, userID int64) (*Subscription, error)
}

type api interface {
	PortalURL(ctx context.Context, customerID, subscriptionID string) (string, error)
	Cancel(ctx context.Context, subscriptionID string, now bool) error
}

type counter interface {
	Count(name events.Name)
}

type Settings struct {
	Secret      string
	ClientToken string
	Monthly     string
	Yearly      string
	Sandbox     bool
	TestUsers   []int64
	BaseURL     string
}

type Handler struct {
	store     subscriptions
	api       api
	settings  Settings
	prices    map[string]string
	testers   map[int64]bool
	templates *htmlutil.Templates
	counts    counter
	logger    *slog.Logger
	now       func() time.Time
}

func NewHandler(store subscriptions, api api, settings Settings, templates *htmlutil.Templates, counts counter, logger *slog.Logger) *Handler {
	settings.BaseURL = strings.TrimRight(settings.BaseURL, "/")

	testers := make(map[int64]bool, len(settings.TestUsers))
	for _, id := range settings.TestUsers {
		testers[id] = true
	}

	return &Handler{
		store:     store,
		api:       api,
		settings:  settings,
		prices:    map[string]string{PlanMonthly: settings.Monthly, PlanYearly: settings.Yearly},
		testers:   testers,
		templates: templates,
		counts:    counts,
		logger:    logger,
		now:       time.Now,
	}
}

func (h *Handler) ours(e Event) bool {
	return e.PriceID == h.settings.Monthly || e.PriceID == h.settings.Yearly
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		http.Error(w, "the body could not be read", http.StatusBadRequest)
		return
	}

	if !Signed(h.settings.Secret, body, r.Header.Get(SignatureHeader), h.now()) {
		h.logger.Warn("billing webhook with a bad or stale signature")
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}

	e, isSubscription, err := ParseEvent(body)
	if err != nil {
		h.logger.Error("billing webhook not understood", "err", err)
		http.Error(w, "payload not understood", http.StatusBadRequest)
		return
	}
	if !isSubscription {
		w.WriteHeader(http.StatusOK)
		return
	}

	args := []any{"event", e.Name, "event_id", e.ID, "subscription", e.SubscriptionID, "status", e.Status}

	if !h.ours(e) {
		h.logger.Warn("billing webhook for another product ignored", append(args, "price", e.PriceID)...)
		w.WriteHeader(http.StatusOK)
		return
	}

	if e.Site != "" && e.Site != h.settings.BaseURL {
		h.logger.Warn("billing webhook for a checkout made on another site ignored", append(args, "site", e.Site)...)
		w.WriteHeader(http.StatusOK)
		return
	}

	if !KnownStatus(e.Status) {
		h.logger.Error("billing webhook with an unknown status", args...)
		http.Error(w, "unknown status", http.StatusUnprocessableEntity)
		return
	}

	access := AccessUntil(e, h.now())

	outcome, userID, err := h.store.Apply(r.Context(), e, access)
	if errors.Is(err, ErrUnknownSubscription) {
		h.logger.Warn("billing webhook for a subscription with no user yet", args...)
		http.Error(w, "subscription not linked yet", http.StatusConflict)
		return
	}
	if err != nil {
		h.logger.Error("billing webhook not applied", append(args, "err", err)...)
		http.Error(w, "not applied", http.StatusInternalServerError)
		return
	}

	h.logger.Info("billing webhook", append(args, "outcome", string(outcome), "user_id", userID, "access_until", access)...)

	if outcome == Applied {
		h.counts.Count(events.Billing(e.Name))
	}

	w.WriteHeader(http.StatusOK)
}

type PayPage struct {
	ClientToken string
	Sandbox     bool
	PriceID     string
	Email       string
	UserID      string
	Site        string
	SuccessURL  string
	BackURL     string
	Transaction string
}

func (h *Handler) Pay(w http.ResponseWriter, r *http.Request) {
	form := PayPage{
		ClientToken: h.settings.ClientToken,
		Sandbox:     h.settings.Sandbox,
		Site:        h.settings.BaseURL,
		SuccessURL:  h.settings.BaseURL + "/account?checkout=done",
		BackURL:     "/pricing",
		Transaction: r.URL.Query().Get("_ptxn"),
	}

	if form.Transaction == "" {
		u := user.GetUserContext(r)
		if u.IsAnonymous() {
			http.Redirect(w, r, "/signin", http.StatusSeeOther)
			return
		}
		if !h.Offered(u.ID) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}

		plan := r.URL.Query().Get("plan")
		price, ok := h.prices[plan]
		if !ok {
			htmlutil.BadRequestPage(w, r, h.logger)
			return
		}

		if !u.Verified {
			session.FromContext(r).Put("flash", user.MsgSubscribeNeedsEmail)
			http.Redirect(w, r, "/account", http.StatusSeeOther)
			return
		}

		sub, err := h.mine(r.Context(), u.ID)
		if err != nil {
			htmlutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
		if sub != nil && (sub.Live(h.now()) || sub.Status == StatusPaused) {
			http.Redirect(w, r, "/billing/portal", http.StatusSeeOther)
			return
		}

		h.counts.Count(events.CheckoutStart(plan))

		form.PriceID = price
		form.Email = u.Email
		form.UserID = strconv.FormatInt(u.ID, 10)
	}

	page := user.NewPage(r, form)
	page.NoIndex = true

	w.Header().Set("Content-Security-Policy", PayPolicy)
	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Pay, page, h.logger)
}

func (h *Handler) Portal(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)

	sub, err := h.mine(r.Context(), u.ID)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}
	if sub == nil || sub.CustomerID == "" {
		http.Redirect(w, r, "/pricing", http.StatusSeeOther)
		return
	}

	to, err := h.api.PortalURL(r.Context(), sub.CustomerID, sub.SubscriptionID)
	if err != nil {
		h.logger.Error("customer portal link not fetched", "err", err)
		session.FromContext(r).Put("flash", MsgPortalDown)
		http.Redirect(w, r, "/account", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (h *Handler) Offered(userID int64) bool {
	return !h.settings.Sandbox || h.testers[userID]
}

func (h *Handler) mine(ctx context.Context, userID int64) (*Subscription, error) {
	sub, err := h.store.Find(ctx, userID)
	if err != nil || sub == nil {
		return nil, err
	}
	if sub.TestMode != h.settings.Sandbox {
		return nil, nil
	}
	return sub, nil
}

func (h *Handler) Plan(ctx context.Context, userID int64) (*user.PaidPlan, error) {
	sub, err := h.mine(ctx, userID)
	if err != nil || sub == nil || !sub.Live(h.now()) {
		return nil, err
	}

	return &user.PaidPlan{
		Status:    sub.Status,
		Yearly:    sub.VariantID == h.settings.Yearly,
		PeriodEnd: sub.PeriodEnd,
	}, nil
}

func (h *Handler) Cancel(ctx context.Context, userID int64) error {
	sub, err := h.mine(ctx, userID)
	if err != nil {
		return err
	}
	if sub == nil || !sub.Open() {
		return nil
	}

	if err = h.api.Cancel(ctx, sub.SubscriptionID, sub.Status == StatusPaused); err != nil {
		return err
	}

	h.logger.Info("subscription cancelled for a deleted account", "subscription", sub.SubscriptionID)
	return nil
}
