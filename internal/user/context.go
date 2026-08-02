package user

import (
	"context"
	"net/http"
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
