package events

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Name string

const (
	UploadOK       Name = "upload_ok"
	UploadRejected Name = "upload_rejected"
	UploadTooLarge Name = "upload_too_large"
)

const (
	FlushInterval = time.Minute
	flushTimeout  = 5 * time.Second
)

type store interface {
	Add(ctx context.Context, day string, name Name, total int64) error
}

type key struct {
	day  string
	name Name
}

type Recorder struct {
	mu      sync.Mutex
	pending map[key]int64
	store   store
	now     func() time.Time
	logger  *slog.Logger
}

func NewRecorder(store store, logger *slog.Logger) *Recorder {
	return &Recorder{
		pending: make(map[key]int64),
		store:   store,
		now:     time.Now,
		logger:  logger,
	}
}

func (r *Recorder) Count(name Name) {
	k := key{day: r.now().UTC().Format(time.DateOnly), name: name}

	r.mu.Lock()
	r.pending[k]++
	r.mu.Unlock()
}

func (r *Recorder) Flush(ctx context.Context) error {
	r.mu.Lock()
	batch := r.pending
	r.pending = make(map[key]int64)
	r.mu.Unlock()

	failed := make(map[key]int64)
	var errs []error

	for k, total := range batch {
		if err := r.store.Add(ctx, k.day, k.name, total); err != nil {
			failed[k] = total
			errs = append(errs, err)
		}
	}

	if len(failed) == 0 {
		return nil
	}

	r.mu.Lock()
	for k, total := range failed {
		r.pending[k] += total
	}
	r.mu.Unlock()

	return errors.Join(errs...)
}

func (r *Recorder) Start(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(FlushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				r.flushLogged("event counts kept in memory, will retry")
			}
		}
	}()
}

func (r *Recorder) Close() {
	r.flushLogged("event counts lost on shutdown")
}

func (r *Recorder) flushLogged(message string) {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	if err := r.Flush(ctx); err != nil {
		r.logger.Warn(message, "err", err)
	}
}
