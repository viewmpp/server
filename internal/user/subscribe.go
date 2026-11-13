package user

import (
	"errors"
	"net/http"
	"server/internal/events"
	"server/internal/htmlutil"
	"server/internal/session"
	"time"
)

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := GetUserContext(r)

	back := h.backTo(r)

	if u.HasSubscription() {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	if !u.Verified {
		sess.Put("flash", MsgSubscribeNeedsEmail)
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	if h.offered(u.ID) {
		h.startTrial(w, r, sess, u, back)
		return
	}

	until := time.Now().Add(h.earlyAccessPeriod)

	seat, err := h.store.GrantSubscription(r.Context(), u.ID, &until, h.earlyAccessSeats)
	if errors.Is(err, ErrSeatLimit) {
		sess.Put("flash", MsgProUnavailable)
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.logger.Info("early access granted", "seat", seat, "of", h.earlyAccessSeats)

	sess.Put("flash", MsgEarlyAccessGranted(seat, until))

	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (h *Handler) startTrial(w http.ResponseWriter, r *http.Request, sess *session.Session, u *User, back string) {
	until := time.Now().Add(h.trialPeriod)

	err := h.store.StartTrial(r.Context(), u.ID, until)
	switch {
	case errors.Is(err, ErrTrialUsed):
		sess.Put("flash", MsgTrialUsed)
		http.Redirect(w, r, "/pricing", http.StatusSeeOther)
		return
	case errors.Is(err, ErrEditConflict):
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	case err != nil:
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.counts.Count(events.TrialStart)
	h.logger.Info("trial started", "user_id", u.ID, "until", until)

	sess.Put("flash", MsgTrialStarted(h.trialPeriod, until))

	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (h *Handler) backTo(r *http.Request) string {
	return htmlutil.BackTo(r, h.baseURL)
}
