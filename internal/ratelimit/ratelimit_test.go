package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func newTestLimiter(t *testing.T, limit int, window time.Duration) *Limiter {
	t.Helper()

	l := New(limit, window)
	t.Cleanup(l.Close)

	return l
}

func TestAllowDoesNotRecord(t *testing.T) {
	l := newTestLimiter(t, 3, time.Minute)

	for i := 0; i < 20; i++ {
		if !l.Allow("k") {
			t.Fatalf("Allow refused after %d checks; checking must not consume the budget", i)
		}
	}

	for i := 0; i < 3; i++ {
		if !l.Take("k") {
			t.Fatalf("Take refused at %d of 3", i)
		}
	}

	if l.Take("k") {
		t.Fatal("Take allowed a fourth request within the limit of 3")
	}
}

func TestAllowDoesNotInflateStaleEntries(t *testing.T) {
	l := newTestLimiter(t, 100, time.Minute)

	l.mu.Lock()
	now := time.Now()
	stale := now.Add(-2 * time.Minute)
	l.hits["k"] = []time.Time{stale, stale, stale, now}
	l.mu.Unlock()

	for i := 0; i < 5; i++ {
		l.Allow("k")
	}

	if got := l.countFresh("k"); got != 1 {
		t.Fatalf("fresh hits = %d after repeated Allow, want 1: checking rewrote stored state", got)
	}
}

func TestTakeEnforcesLimit(t *testing.T) {
	l := newTestLimiter(t, 5, time.Minute)

	served := 0
	for i := 0; i < 50; i++ {
		if l.Take("k") {
			served++
		}
	}

	if served != 5 {
		t.Fatalf("served %d requests, want 5", served)
	}
}

func TestTakeIsAtomicUnderConcurrency(t *testing.T) {
	const limit = 10

	l := newTestLimiter(t, limit, time.Minute)

	var wg sync.WaitGroup
	var mu sync.Mutex
	served := 0

	start := make(chan struct{})

	for i := 0; i < 400; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			<-start

			if !l.Take("k") {
				return
			}

			time.Sleep(time.Millisecond)

			mu.Lock()
			served++
			mu.Unlock()
		}()
	}

	close(start)
	wg.Wait()

	if served != limit {
		t.Fatalf("served %d concurrent requests, want %d", served, limit)
	}
}

func TestTakeAllRecordsEveryKeyOnlyWhenAllPass(t *testing.T) {
	l := newTestLimiter(t, 2, time.Minute)

	keys := []string{"ip", "user"}

	for i := 0; i < 2; i++ {
		if _, ok := l.TakeAll(keys); !ok {
			t.Fatalf("TakeAll refused at %d of 2", i)
		}
	}

	key, ok := l.TakeAll(keys)
	if ok {
		t.Fatal("TakeAll allowed a third request within the limit of 2")
	}
	if key != "ip" {
		t.Fatalf("blamed key = %q, want %q", key, "ip")
	}

	l.Reset("ip")

	if _, ok := l.TakeAll(keys); ok {
		t.Fatal("TakeAll passed while the second key was still exhausted")
	}
}

func TestTakeAllLeavesBudgetAloneWhenRefused(t *testing.T) {
	l := newTestLimiter(t, 2, time.Minute)

	l.Count("blocked")
	l.Count("blocked")

	for i := 0; i < 5; i++ {
		if _, ok := l.TakeAll([]string{"free", "blocked"}); ok {
			t.Fatal("TakeAll passed on an exhausted key")
		}
	}

	if got := l.countFresh("free"); got != 0 {
		t.Fatalf("recorded %d hits against the healthy key of a refused request, want 0", got)
	}
}

func TestHitsExpireWithTheWindow(t *testing.T) {
	l := newTestLimiter(t, 1, 60*time.Millisecond)

	if !l.Take("k") {
		t.Fatal("first request refused")
	}
	if l.Take("k") {
		t.Fatal("second request allowed inside the window")
	}

	time.Sleep(90 * time.Millisecond)

	if !l.Take("k") {
		t.Fatal("request refused after the window had passed")
	}
}

func TestResetClearsFailures(t *testing.T) {
	l := newTestLimiter(t, 2, time.Minute)

	l.Count("k")
	l.Count("k")

	if l.Allow("k") {
		t.Fatal("Allow passed on an exhausted key")
	}

	l.Reset("k")

	if !l.Allow("k") {
		t.Fatal("Allow refused after Reset")
	}
}

func (l *Limiter) countFresh(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.fresh(key))
}
