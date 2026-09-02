package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/clientip"
	"server/internal/ratelimit"
	"server/internal/session"
	"server/internal/user"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()

	notice := ratelimit.New(1, throttleNoticeWindow)
	t.Cleanup(notice.Close)

	address := ratelimit.New(1000, time.Minute)
	t.Cleanup(address.Close)

	return &Server{
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		addressLimiter: address,
		throttleNotice: notice,
	}
}

func read(t *testing.T, h http.HandlerFunc, path, ip string) *httptest.ResponseRecorder {
	t.Helper()

	return readAs(t, h, path, ip, user.AnonymousUser)
}

func readAs(t *testing.T, h http.HandlerFunc, path, ip string, u *user.User) *httptest.ResponseRecorder {
	t.Helper()

	r := clientip.SetContext(httptest.NewRequest(http.MethodGet, path, nil), ip)
	w := httptest.NewRecorder()

	store := session.NewStore(nil, time.Hour, false, "", slog.New(slog.NewTextHandler(io.Discard, nil)))

	sess, err := store.New(w, r)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	h(w, user.SetUserContext(session.SetContext(r, sess), u))

	return w
}

func TestReadsAreThrottledPerAddress(t *testing.T) {
	limiter := ratelimit.New(2, time.Minute)
	t.Cleanup(limiter.Close)

	s := newTestServer(t)
	h := s.throttle(limiter, "read-ip:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 1; i <= 2; i++ {
		if code := read(t, h, "/p/abc", "203.0.113.5").Code; code != http.StatusOK {
			t.Fatalf("read %d = %d, want 200: the limit must not bite before it is reached", i, code)
		}
	}

	res := read(t, h, "/p/abc", "203.0.113.5")
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("read 3 = %d, want 429", res.Code)
	}

	if got := res.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want %q", got, "60")
	}

	if code := read(t, h, "/p/abc", "198.51.100.7").Code; code != http.StatusOK {
		t.Errorf("another address = %d, want 200: one visitor must not shut out the rest", code)
	}
}

func TestThrottledReadAnswersInTheFormatOfTheRoute(t *testing.T) {
	limiter := ratelimit.New(0, time.Minute)
	t.Cleanup(limiter.Close)

	s := newTestServer(t)
	h := s.throttle(limiter, "read-ip:", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("the handler ran although the limit was exhausted")
	})

	page := read(t, h, "/p/abc", "203.0.113.5")
	if ct := page.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("page Content-Type = %q, want html: a browser must not be handed raw JSON", ct)
	}

	api := read(t, h, "/api/v1/projects/abc", "203.0.113.5")
	if ct := api.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("api Content-Type = %q, want json: the viewer parses this response", ct)
	}
}

func TestReadAndExportAreCountedApart(t *testing.T) {
	reads := ratelimit.New(1, time.Minute)
	exports := ratelimit.New(1, time.Minute)
	t.Cleanup(reads.Close)
	t.Cleanup(exports.Close)

	s := newTestServer(t)
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

	page := s.throttle(reads, "read-ip:", ok)
	export := s.throttle(exports, "export-ip:", ok)

	read(t, page, "/p/abc", "203.0.113.5")

	if code := read(t, export, "/p/abc/xlsx", "203.0.113.5").Code; code != http.StatusOK {
		t.Errorf("export = %d, want 200: opening a plan must not spend the export budget", code)
	}
}

func TestEveryUnmeteredReadRouteIsThrottled(t *testing.T) {
	exhausted := ratelimit.New(0, time.Minute)
	t.Cleanup(exhausted.Close)

	s := newTestServer(t)
	s.readLimiter = exhausted
	s.exportLimiter = exhausted

	mux := s.mux()

	paths := []string{
		"/p/abc",
		"/p/abc/xlsx",
		"/api/v1/projects/abc",
		"/api/v1/examples/sample",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := clientip.SetContext(httptest.NewRequest(http.MethodGet, path, nil), "203.0.113.5")

			store := session.NewStore(nil, time.Hour, false, "", slog.New(slog.NewTextHandler(io.Discard, nil)))

			sess, err := store.New(w, r)
			if err != nil {
				t.Fatalf("new session: %v", err)
			}

			mux.ServeHTTP(w, user.SetUserContext(session.SetContext(r, sess), user.AnonymousUser))

			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("%s = %d, want 429: this route reads and decompresses a plan on every request", path, w.Code)
			}
		})
	}
}

func TestSignedInVisitorsShareAnAddressButNotABudget(t *testing.T) {
	limiter := ratelimit.New(1, time.Minute)
	t.Cleanup(limiter.Close)

	s := newTestServer(t)
	h := s.throttle(limiter, "read-ip:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	office := "203.0.113.5"
	ann := &user.User{ID: 1, Subscription: user.SubscriptionFree}
	bob := &user.User{ID: 2, Subscription: user.SubscriptionFree}

	if code := readAs(t, h, "/p/abc", office, ann).Code; code != http.StatusOK {
		t.Fatalf("first read = %d, want 200", code)
	}

	if code := readAs(t, h, "/p/abc", office, ann).Code; code != http.StatusTooManyRequests {
		t.Fatalf("second read by the same person = %d, want 429", code)
	}

	if code := readAs(t, h, "/p/abc", office, bob).Code; code != http.StatusOK {
		t.Errorf("colleague behind the same address = %d, want 200: one visitor must not shut out the office", code)
	}
}
