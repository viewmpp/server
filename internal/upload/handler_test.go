package upload

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/clientip"
	"server/internal/events"
	"server/internal/fixtures"
	"server/internal/parser"
	"server/internal/ratelimit"
	"server/internal/user"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type tally struct {
	mu     sync.Mutex
	counts map[events.Name]int
}

func (t *tally) Count(name events.Name) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[name]++
}

func (t *tally) get(name events.Name) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[name]
}

type noStore struct{}

func (noStore) Save(context.Context, int64, string, []byte) (string, error) {
	return "", nil
}

type sidecar struct {
	calls    atomic.Int32
	received atomic.Int64
}

func newSidecar(t *testing.T) (*sidecar, *parser.Client) {
	t.Helper()

	s := &sidecar{}
	plan := fixtures.All()["mpp8.json"]

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.calls.Add(1)
		n, _ := io.Copy(io.Discard, r.Body)
		s.received.Store(n)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(plan)
	}))
	t.Cleanup(srv.Close)

	return s, &parser.Client{URL: srv.URL, HTTP: srv.Client(), ParseTimeout: 50 * time.Millisecond}
}

func newHandler(t *testing.T, client *parser.Client, counts *tally) *Handler {
	t.Helper()

	limiter := ratelimit.New(100, time.Minute)
	t.Cleanup(limiter.Close)

	return NewHandler(client, noStore{}, limiter, counts, slog.New(slog.DiscardHandler))
}

func upload(h *Handler, body io.Reader, size int64) *httptest.ResponseRecorder {
	return uploadFrom(h, body, size, "")
}

func uploadFrom(h *Handler, body io.Reader, size int64, referer string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/upload?name=plan.mpp", body)
	r.ContentLength = size
	if referer != "" {
		r.Header.Set("Referer", referer)
	}
	r = clientip.SetContext(r, "203.0.113.5")
	r = user.SetUserContext(r, user.AnonymousUser)

	rec := httptest.NewRecorder()
	h.Upload(rec, r)

	return rec
}

type trickle struct {
	left  int
	chunk int
	delay time.Duration
}

func (t *trickle) Read(p []byte) (int, error) {
	if t.left == 0 {
		return 0, io.EOF
	}

	time.Sleep(t.delay)

	n := min(t.chunk, t.left, len(p))
	for i := range n {
		p[i] = 'x'
	}
	t.left -= n

	return n, nil
}

type cutOff struct {
	left int
}

func (c *cutOff) Read(p []byte) (int, error) {
	if c.left == 0 {
		return 0, io.ErrUnexpectedEOF
	}

	n := min(c.left, len(p))
	c.left -= n

	return n, nil
}

func TestASlowUploadDoesNotSpendTheParseTimeout(t *testing.T) {
	side, client := newSidecar(t)
	counts := &tally{counts: map[events.Name]int{}}

	const size = 3000
	body := &trickle{left: size, chunk: 1000, delay: 40 * time.Millisecond}

	rec := upload(newHandler(t, client, counts), body, size)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: the upload took longer than the %v parse timeout and was cut off: %s", rec.Code, client.ParseTimeout, rec.Body.String())
	}
	if got := side.received.Load(); got != size {
		t.Errorf("parser received %d bytes, want %d", got, size)
	}
	if got := counts.get(events.UploadOK); got != 1 {
		t.Errorf("upload_ok counted %d times, want 1", got)
	}
}

func TestAnUploadIsCountedAgainstThePageItCameFrom(t *testing.T) {
	tests := map[string]struct {
		referer string
		page    string
	}{
		"a landing page":          {"https://viewmpp.com/mpp-to-excel", "/mpp-to-excel"},
		"the home page":           {"https://viewmpp.com/", "/"},
		"the project list":        {"https://viewmpp.com/projects", "/projects"},
		"a page with a query":     {"https://viewmpp.com/open-mpp-file-on-mac?utm_source=x", "/open-mpp-file-on-mac"},
		"an unknown path":         {"https://viewmpp.com/anything/else", events.OtherPage},
		"no referer at all":       {"", events.OtherPage},
		"a referer that is noise": {"::not a url", events.OtherPage},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, client := newSidecar(t)
			counts := &tally{counts: map[events.Name]int{}}

			const size = 10
			rec := uploadFrom(newHandler(t, client, counts), &trickle{left: size, chunk: size}, size, tt.referer)

			if rec.Code != http.StatusOK {
				t.Fatalf("got %d, want 200", rec.Code)
			}
			if got := counts.get(events.UploadFrom(tt.page)); got != 1 {
				t.Errorf("upload_from:%s counted %d times, want 1 (counts %v)", tt.page, got, counts.counts)
			}
		})
	}
}

func TestAnInterruptedUploadIsNotAServerError(t *testing.T) {
	side, client := newSidecar(t)
	counts := &tally{counts: map[events.Name]int{}}

	rec := upload(newHandler(t, client, counts), &cutOff{left: 1000}, 3000)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
	if side.calls.Load() != 0 {
		t.Error("a partial file was sent to the parser")
	}
	if got := counts.get(events.UploadInterrupted); got != 1 {
		t.Errorf("upload_interrupted counted %d times, want 1", got)
	}
}
