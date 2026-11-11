package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type fakeStore struct {
	mu     sync.Mutex
	totals map[key]int64
	fail   map[Name]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{totals: make(map[key]int64), fail: make(map[Name]bool)}
}

func (s *fakeStore) Add(_ context.Context, day string, name Name, total int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fail[name] {
		return errors.New("database is down")
	}

	s.totals[key{day: day, name: name}] += total

	return nil
}

func (s *fakeStore) total(day string, name Name) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.totals[key{day: day, name: name}]
}

func newTestRecorder(store store, at time.Time) *Recorder {
	r := NewRecorder(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.now = func() time.Time { return at }
	return r
}

var today = time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)

func TestCountsReachTheStoreOnFlush(t *testing.T) {
	store := newFakeStore()
	r := newTestRecorder(store, today)

	r.Count(UploadOK)
	r.Count(UploadOK)
	r.Count(UploadRejected)

	if err := r.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := store.total("2026-10-03", UploadOK); got != 2 {
		t.Errorf("upload_ok = %d, want 2", got)
	}
	if got := store.total("2026-10-03", UploadRejected); got != 1 {
		t.Errorf("upload_rejected = %d, want 1", got)
	}
}

func TestAFlushAddsRatherThanReplaces(t *testing.T) {
	store := newFakeStore()
	r := newTestRecorder(store, today)

	for _, batch := range []int{3, 2, 5} {
		for range batch {
			r.Count(UploadOK)
		}
		if err := r.Flush(context.Background()); err != nil {
			t.Fatalf("flush: %v", err)
		}
	}

	if got := store.total("2026-10-03", UploadOK); got != 10 {
		t.Errorf("upload_ok = %d, want 10: each minute adds to the day, it does not overwrite it", got)
	}
}

func TestAFailedFlushKeepsTheCountsForTheNextOne(t *testing.T) {
	store := newFakeStore()
	store.fail[UploadOK] = true
	r := newTestRecorder(store, today)

	r.Count(UploadOK)
	r.Count(UploadOK)

	if err := r.Flush(context.Background()); err == nil {
		t.Fatal("a failing store was reported as a successful flush")
	}

	r.Count(UploadOK)
	store.fail[UploadOK] = false

	if err := r.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := store.total("2026-10-03", UploadOK); got != 3 {
		t.Errorf("upload_ok = %d, want 3: counts from a minute the database was down must not be lost", got)
	}
}

func TestOnlyTheFailedCountsAreRetried(t *testing.T) {
	store := newFakeStore()
	store.fail[UploadRejected] = true
	r := newTestRecorder(store, today)

	r.Count(UploadOK)
	r.Count(UploadRejected)

	_ = r.Flush(context.Background())

	store.fail[UploadRejected] = false

	if err := r.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := store.total("2026-10-03", UploadOK); got != 1 {
		t.Errorf("upload_ok = %d, want 1: what was already written must not be written twice", got)
	}
	if got := store.total("2026-10-03", UploadRejected); got != 1 {
		t.Errorf("upload_rejected = %d, want 1", got)
	}
}

func TestCountsLandOnTheDayTheyHappened(t *testing.T) {
	store := newFakeStore()
	r := newTestRecorder(store, time.Date(2026, time.October, 3, 23, 59, 59, 0, time.UTC))

	r.Count(UploadOK)

	r.now = func() time.Time { return time.Date(2026, time.October, 4, 0, 0, 1, 0, time.UTC) }
	r.Count(UploadOK)

	if err := r.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if store.total("2026-10-03", UploadOK) != 1 || store.total("2026-10-04", UploadOK) != 1 {
		t.Error("a flush that straddles midnight moved a count onto the wrong day")
	}
}

func TestTheDayIsTakenInUTC(t *testing.T) {
	store := newFakeStore()
	almaty := time.FixedZone("UTC+5", 5*60*60)
	r := newTestRecorder(store, time.Date(2026, time.October, 4, 2, 0, 0, 0, almaty))

	r.Count(UploadOK)
	_ = r.Flush(context.Background())

	if store.total("2026-10-03", UploadOK) != 1 {
		t.Error("the day must not depend on the server's time zone")
	}
}

func TestConcurrentCountsAreNotLost(t *testing.T) {
	store := newFakeStore()
	r := newTestRecorder(store, today)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				r.Count(UploadOK)
			}
		}()
	}
	wg.Wait()

	if err := r.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := store.total("2026-10-03", UploadOK); got != 10000 {
		t.Errorf("upload_ok = %d, want 10000: parallel requests lost counts", got)
	}
}

func TestCloseWritesWhatIsLeft(t *testing.T) {
	store := newFakeStore()
	r := newTestRecorder(store, today)

	r.Count(UploadTooLarge)
	r.Close()

	if got := store.total("2026-10-03", UploadTooLarge); got != 1 {
		t.Errorf("upload_too_large = %d, want 1: a deploy must not lose the last minute", got)
	}
}
