package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/parser"
	"server/internal/postgres"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const unreachableDSN = "postgres://nobody:nobody@127.0.0.1:1/nothing?sslmode=disable&connect_timeout=1"

func deadDatabase(t *testing.T) *postgres.Postgres {
	t.Helper()

	db, err := sql.Open("pgx", unreachableDSN)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return &postgres.Postgres{DB: db}
}

func healthServer(t *testing.T, parserURL string, pg *postgres.Postgres) *Server {
	t.Helper()

	return &Server{
		Services: Services{
			client: &parser.Client{URL: parserURL, HTTP: &http.Client{Timeout: time.Second}},
			pg:     pg,
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}
}

func readHealth(t *testing.T, s *Server) healthcheck {
	t.Helper()

	w := httptest.NewRecorder()
	s.serverHealthcheck(w, httptest.NewRequest(http.MethodGet, "/api/v1/healthcheck", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: the endpoint answers 200 whatever the dependencies say", w.Code)
	}

	var hc healthcheck
	if err := json.Unmarshal(w.Body.Bytes(), &hc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return hc
}

func TestTheServerCallsItselfHealthyWhileItAnswers(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	hc := readHealth(t, healthServer(t, stub.URL, deadDatabase(t)))

	if hc.Status != healthy {
		t.Errorf("status = %q, want %q: the process is answering, which is what this field means", hc.Status, healthy)
	}

	if hc.Database != unhealthy {
		t.Errorf("database = %q, want %q", hc.Database, unhealthy)
	}

	if hc.Parser != unhealthy {
		t.Errorf("parser = %q, want %q", hc.Parser, unhealthy)
	}
}

func TestADependencyIsProbedOnceWithinTheWindow(t *testing.T) {
	probes := 0

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probes++
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	t.Cleanup(stub.Close)

	s := healthServer(t, stub.URL, deadDatabase(t))

	for range 5 {
		if got := readHealth(t, s).Parser; got != healthy {
			t.Fatalf("parser = %q, want %q", got, healthy)
		}
	}

	if probes != 1 {
		t.Errorf("the parser was probed %d times for 5 requests: an unthrottled public endpoint must not pass its load downstream", probes)
	}
}

func TestAStaleReportIsProbedAgain(t *testing.T) {
	probes := 0

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probes++
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	t.Cleanup(stub.Close)

	s := healthServer(t, stub.URL, deadDatabase(t))

	readHealth(t, s)

	s.health.mu.Lock()
	s.health.probedAt = time.Now().Add(-healthReportTTL - time.Second)
	s.health.mu.Unlock()

	readHealth(t, s)

	if probes != 2 {
		t.Errorf("probes = %d, want 2: a report older than the window must be taken again", probes)
	}
}
