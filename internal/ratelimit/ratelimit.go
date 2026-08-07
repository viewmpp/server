package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	limit   int
	window  time.Duration
	stopped chan struct{}
}

func New(limit int, window time.Duration) *Limiter {
	l := &Limiter{
		hits:    make(map[string][]time.Time),
		limit:   limit,
		window:  window,
		stopped: make(chan struct{}),
	}

	go l.sweep()

	return l
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

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
