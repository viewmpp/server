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
	"strings"
	"time"
)

const (
	SignatureHeader = "X-Signature"
	maxWebhookBytes = 1 << 20
)

const (
	PlanMonthly = "monthly"
	PlanYearly  = "yearly"
)

const (
	MsgPaymentsDown = "Payments are unavailable right now. Try again in a few minutes"
	MsgPortalDown   = "Subscription management is unavailable right now. Try again in a few minutes"
)

type subscriptions interface {
	Apply(ctx context.Context, e Event, access time.Time) (Outcome, int64, error)
	Find(ctx context.Context, userID int64) (*Subscription, error)
}

type api interface {
	CreateCheckout(ctx context.Context, ch Checkout) (string, error)
	PortalURL(ctx context.Context, subscriptionID string) (string, error)
	Cancel(ctx context.Context, subscriptionID string) error
}

type counter interface {
	Count(name events.Name)
}

type Settings struct {
	Secret    string
	StoreID   string
	Monthly   string
	Yearly    string
	TestMode  bool
	TestUsers []int64
	BaseURL   string
}

type Handler struct {
	store    subscriptions
	api      api
	settings Settings
	variants map[string]string
	testers  map[int64]bool
	counts   counter
	logger   *slog.Logger
	now      func() time.Time
}

func NewHandler(store subscriptions, api api, settings Settings, counts counter, logger *slog.Logger) *Handler {
	settings.BaseURL = strings.TrimRight(settings.BaseURL, "/")

	testers := make(map[int64]bool, len(settings.TestUsers))
	for _, id := range settings.TestUsers {
		testers[id] = true
	}

	return &Handler{
		testers:  testers,
		store:    store,
		api:      api,
		settings: settings,
		variants: map[string]string{PlanMonthly: settings.Monthly, PlanYearly: settings.Yearly},
		counts:   counts,
		logger:   logger,
		now:      time.Now,
	}
}

func (h *Handler) ours(e Event) bool {
	if e.StoreID != h.settings.StoreID || e.TestMode != h.settings.TestMode {
		return false
	}
	return e.VariantID == h.settings.Monthly || e.VariantID == h.settings.Yearly
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		http.Error(w, "the body could not be read", http.StatusBadRequest)
		return
	}

	if !Signed(h.settings.Secret, body, r.Header.Get(SignatureHeader)) {
		h.logger.Warn("billing webhook with a bad signature")
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

	args := []any{"event", e.Name, "subscription", e.SubscriptionID, "status", e.Status}

	if !h.ours(e) {
		h.logger.Warn("billing webhook for another product ignored",
			append(args, "store", e.StoreID, "variant", e.VariantID, "test_mode", e.TestMode)...)
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

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	plan := r.PostForm.Get("plan")
	variant, ok := h.variants[plan]
	if !ok {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	u := user.GetUserContext(r)
	if !h.Offered(u.ID) {
		htmlutil.NotFoundPage(w, r, h.logger)
		return
	}

	back := htmlutil.BackTo(r, h.settings.BaseURL)

	if !u.Verified {
		sess.Put("flash", user.MsgSubscribeNeedsEmail)
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	sub, err := h.mine(r.Context(), u.ID)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}
	if sub != nil && (sub.Live(h.now()) || sub.Status == "paused") {
		http.Redirect(w, r, "/billing/portal", http.StatusSeeOther)
		return
	}

	h.counts.Count(events.CheckoutStart(plan))

	to, err := h.api.CreateCheckout(r.Context(), Checkout{
		VariantID:  variant,
		UserID:     u.ID,
		Email:      u.Email,
		Site:       h.settings.BaseURL,
		RedirectTo: h.settings.BaseURL + "/account?checkout=done",
	})
	if err != nil {
		h.logger.Error("checkout not created", "plan", plan, "err", err)
		sess.Put("flash", MsgPaymentsDown)
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (h *Handler) Portal(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)

	sub, err := h.mine(r.Context(), u.ID)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}
	if sub == nil {
		http.Redirect(w, r, "/pricing", http.StatusSeeOther)
		return
	}

	to, err := h.api.PortalURL(r.Context(), sub.SubscriptionID)
	if err != nil {
		h.logger.Error("customer portal link not fetched", "err", err)
		session.FromContext(r).Put("flash", MsgPortalDown)
		http.Redirect(w, r, "/account", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (h *Handler) Offered(userID int64) bool {
	return !h.settings.TestMode || h.testers[userID]
}

func (h *Handler) mine(ctx context.Context, userID int64) (*Subscription, error) {
	sub, err := h.store.Find(ctx, userID)
	if err != nil || sub == nil {
		return nil, err
	}
	if sub.TestMode != h.settings.TestMode {
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

	if err = h.api.Cancel(ctx, sub.SubscriptionID); err != nil {
		return err
	}

	h.logger.Info("subscription cancelled for a deleted account", "subscription", sub.SubscriptionID)
	return nil
}
