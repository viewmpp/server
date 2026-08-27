package ratelimit

import (
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

func (l *Limiter) Take(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.allow(key) {
		return false
	}

	l.count(key)

	return true
}

func (l *Limiter) TakeAll(keys []string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		if !l.allow(key) {
			return key, false
		}
	}

	for _, key := range keys {
		l.count(key)
	}

	return "", true
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.allow(key)
}

func (l *Limiter) AllowAll(keys []string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		if !l.allow(key) {
			return key, false
		}
	}

	return "", true
}

func (l *Limiter) Count(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.count(key)
}

func (l *Limiter) CountAll(keys []string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		l.count(key)
	}
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.hits, key)
}

func (l *Limiter) ResetAll(keys []string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		delete(l.hits, key)
	}
}

func (l *Limiter) Window() time.Duration {
	return l.window
}

func (l *Limiter) Close() {
	close(l.stopped)
}

func (l *Limiter) allow(key string) bool {
	return len(l.fresh(key)) < l.limit
}

func (l *Limiter) count(key string) {
	l.hits[key] = append(l.fresh(key), time.Now())
}

func (l *Limiter) fresh(key string) []time.Time {
	hits := l.hits[key]
	cutoff := time.Now().Add(-l.window)

	i := 0
	for i < len(hits) && !hits[i].After(cutoff) {
		i++
	}

	return hits[i:]
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
				if kept := l.fresh(key); len(kept) == 0 {
					delete(l.hits, key)
				} else {
					l.hits[key] = kept
				}
			}
			l.mu.Unlock()
		}
	}
}
