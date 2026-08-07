package session

import (
	"context"
	"net/http"
)

type contextKey string

const sessionContextKey = contextKey("session")

func SetContext(r *http.Request, sess *Session) *http.Request {
	ctx := context.WithValue(r.Context(), sessionContextKey, sess)
	return r.WithContext(ctx)
}

func FromContext(r *http.Request) *Session {
	sess, ok := r.Context().Value(sessionContextKey).(*Session)
	if !ok {
		panic("missing session value in request context")
	}
	return sess
}
