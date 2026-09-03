package clientip

import (
	"context"
	"net/http"

	"github.com/realclientip/realclientip-go"
)

type contextKey string

const clientIPContextKey = contextKey("clientip")

type Resolver struct {
	strategy realclientip.Strategy
}

func NewResolver(proxies int) (*Resolver, error) {
	strategy, err := strategyFor(proxies)
	if err != nil {
		return nil, err
	}

	return &Resolver{strategy: strategy}, nil
}

func strategyFor(proxies int) (realclientip.Strategy, error) {
	if proxies <= 0 {
		return realclientip.RemoteAddrStrategy{}, nil
	}

	trusted, err := realclientip.NewRightmostTrustedCountStrategy("X-Forwarded-For", proxies)
	if err != nil {
		return nil, err
	}

	return realclientip.NewChainStrategy(trusted, realclientip.RemoteAddrStrategy{}), nil
}

func (res *Resolver) Get(r *http.Request) string {
	return res.strategy.ClientIP(r.Header, r.RemoteAddr)
}

func SetContext(r *http.Request, ip string) *http.Request {
	ctx := context.WithValue(r.Context(), clientIPContextKey, ip)
	return r.WithContext(ctx)
}

func From(r *http.Request) string {
	ip, ok := r.Context().Value(clientIPContextKey).(string)
	if !ok {
		panic("missing client ip value in request context")
	}
	return ip
}
