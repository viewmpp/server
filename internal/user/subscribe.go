package user

import (
	"net/http"
	"server/internal/htmlutil"
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

	back := r.Header.Get("Referer")
	if back == "" {
		back = "/"
	}

	if u.HasSubscription() {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	if !u.Verified {
		sess.Put("flash", MsgSubscribeNeedsEmail)
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	taken, err := h.store.CountSubscribers(r.Context())
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if taken >= h.earlyAccessSeats {
		sess.Put("flash", MsgEarlyAccessClosed)
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	if err = h.store.GrantSubscription(r.Context(), u.ID, nil); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.logger.Info("early access granted", "user_id", u.ID, "seat", taken+1, "of", h.earlyAccessSeats)

	sess.Put("flash", MsgEarlyAccessGranted(taken+1))

	http.Redirect(w, r, back, http.StatusSeeOther)
}
