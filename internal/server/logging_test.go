package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAFailedRequestStillReachesTheAccessLog(t *testing.T) {
	buf := new(bytes.Buffer)
	s := &Server{logger: slog.New(slog.NewTextHandler(buf, nil))}

	h := s.logRequest(s.recoverPanic(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p/abc", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("client got %d, want 500", w.Code)
	}

	logged := buf.String()

	if !strings.Contains(logged, `msg="request handled"`) {
		t.Error("a request that panicked left no access log line: crashes would be missing from every request count")
	}

	if !strings.Contains(logged, "status_code=500") {
		t.Errorf("access log did not record the status the client received:\n%s", logged)
	}
}
