package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/realclientip/realclientip-go"
)

type Limiter struct {
	mu       sync.Mutex
	hits     map[string][]time.Time
	limit    int
	window   time.Duration
	strategy realclientip.Strategy
	stopped  chan struct{}
}

func New(limit int, window time.Duration, proxies int) (*Limiter, error) {
	strategy, err := clientIPStrategy(proxies)
	if err != nil {
		return nil, err
	}

	l := &Limiter{
		hits:     make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		strategy: strategy,
		stopped:  make(chan struct{}),
	}

	go l.sweep()

	return l, nil
}

func clientIPStrategy(proxies int) (realclientip.Strategy, error) {
	if proxies <= 0 {
		return realclientip.RemoteAddrStrategy{}, nil
	}

	trusted, err := realclientip.NewRightmostTrustedCountStrategy("X-Forwarded-For", proxies)
	if err != nil {
		return nil, err
	}

	return realclientip.NewChainStrategy(trusted, realclientip.RemoteAddrStrategy{}), nil
}

func (l *Limiter) ClientIP(r *http.Request) string {
	return l.strategy.ClientIP(r.Header, r.RemoteAddr)
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.fresh(key)) < l.limit
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.hits[key] = append(l.fresh(key), time.Now())
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.hits, key)
}

func (l *Limiter) AllowAll(keys []string) (string, bool) {
	for _, key := range keys {
		if !l.Allow(key) {
			return key, false
		}
	}
	return "", true
}

func (l *Limiter) FailAll(keys []string) {
	for _, key := range keys {
		l.Fail(key)
	}
}

func (l *Limiter) ResetAll(keys []string) {
	for _, key := range keys {
		l.Reset(key)
	}
}

func (l *Limiter) Close() {
	close(l.stopped)
}

func (l *Limiter) fresh(key string) []time.Time {
	cutoff := time.Now().Add(-l.window)

	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	return kept
}

func (l *Limiter) sweep() {
	ticker := time.NewTicker(l.window)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopped:
			return
		case <-ticker.C:
			l.mu.Lock()
			for key := range l.hits {
				if len(l.fresh(key)) == 0 {
					delete(l.hits, key)
				} else {
					l.hits[key] = l.fresh(key)
				}
			}
			l.mu.Unlock()
		}
	}
}

