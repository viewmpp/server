package user

import (
	"context"
	"net/http"
	"server/internal/htmlutil"
)

type contextKey string

const userContextKey = contextKey("user")

func SetUserContext(r *http.Request, user *User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func GetUserContext(r *http.Request) *User {
	user, ok := r.Context().Value(userContextKey).(*User)
	if !ok {
		panic("missing user value in request context")
	}
	return user
}

func NewPage(r *http.Request, form any) htmlutil.Page {
	u := GetUserContext(r)

	page := htmlutil.Page{Form: form, MaxUpload: u.MaxUploadBytes()}
	if !u.IsAnonymous() {
		page.UserEmail = u.Email
		page.Verified = u.Verified
		page.Pro = u.HasSubscription()
		page.ProWarning = proWarning(u)
	}

	return page
}
