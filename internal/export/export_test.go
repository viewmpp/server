package export

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/clientip"
	"server/internal/fixtures"
	"server/internal/ratelimit"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func newHandler(t *testing.T, limit int) *Handler {
	t.Helper()

	limiter := ratelimit.New(limit, time.Minute)
	t.Cleanup(limiter.Close)

	return NewHandler(limiter, slog.New(slog.DiscardHandler))
}

func post(t *testing.T, h *Handler, body []byte, name string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/xlsx?name="+name, bytes.NewReader(body))
	r.RemoteAddr = "203.0.113.5:1234"
	r = clientip.SetContext(r, "203.0.113.5")

	rec := httptest.NewRecorder()
	h.XLSX(rec, r)

	return rec
}

func TestExportsEveryFixture(t *testing.T) {
	for name, raw := range fixtures.All() {
		t.Run(name, func(t *testing.T) {
			rec := post(t, newHandler(t, 100), raw, "plan.mpp")

			if rec.Code != http.StatusOK {
				t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
			}

			f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
			if err != nil {
				t.Fatalf("result is not a workbook: %v", err)
			}
			defer func() { _ = f.Close() }()

			if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="plan.xlsx"; filename*=UTF-8''plan.xlsx` {
				t.Errorf("unexpected disposition %q", got)
			}
		})
	}
}

func TestRejectsGarbage(t *testing.T) {
	cases := map[string]string{
		"not json":      `hello`,
		"unknown field": `{"contract_version":1,"surprise":true}`,
		"other version": `{"contract_version":2}`,
		"empty body":    ``,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if rec := post(t, newHandler(t, 100), []byte(body), "x.mpp"); rec.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400", rec.Code)
			}
		})
	}
}

func TestRejectsOversizedBody(t *testing.T) {
	huge := bytes.Repeat([]byte("a"), MaxContractBytes+1)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/xlsx", bytes.NewReader(huge))
	r.RemoteAddr = "203.0.113.9:1234"
	r.ContentLength = int64(len(huge))
	r = clientip.SetContext(r, "203.0.113.9")

	rec := httptest.NewRecorder()
	newHandler(t, 100).XLSX(rec, r)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d, want 413", rec.Code)
	}
}

func TestRateLimited(t *testing.T) {
	h := newHandler(t, 2)
	raw := fixtures.All()["mpp8.json"]

	for i := 1; i <= 2; i++ {
		if rec := post(t, h, raw, "a.mpp"); rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: got %d, want 200", i, rec.Code)
		}
	}

	if rec := post(t, h, raw, "a.mpp"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("third attempt: got %d, want 429", rec.Code)
	}
}
