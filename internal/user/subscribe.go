package user

import (
	"errors"
	"net/http"
	"net/url"
	"server/internal/htmlutil"
	"strings"
	"time"
)

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

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

	h.logger.Info("early access granted", "user_id", u.ID, "seat", seat, "of", h.earlyAccessSeats)

	sess.Put("flash", MsgEarlyAccessGranted(seat, until))

	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (h *Handler) backTo(r *http.Request) string {
	reference, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || reference.Scheme+"://"+reference.Host != strings.TrimSuffix(h.baseURL, "/") {
		return "/"
	}

	return reference.RequestURI()
}
