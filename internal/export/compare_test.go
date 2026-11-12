package export

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"server/internal/clientip"
	"server/internal/events"
	"strings"
	"testing"
)

func comparePlans(t *testing.T, h *Handler, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/compare", bytes.NewReader(body))
	r = clientip.SetContext(r, "203.0.113.5")

	rec := httptest.NewRecorder()
	h.Compare(rec, r)

	return rec
}

func versions(t *testing.T) []byte {
	t.Helper()

	read := func(name string) json.RawMessage {
		raw, err := os.ReadFile(filepath.Join("..", "diff", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	body, err := json.Marshal(map[string]json.RawMessage{
		"previous": read("hospital-v1.json"),
		"current":  read("hospital-v2.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestComparesTwoVersionsAndCountsIt(t *testing.T) {
	counts := tally{}

	rec := comparePlans(t, newCountingHandler(t, 100, counts), versions(t))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var report struct {
		Finish struct {
			Minutes int64 `json:"minutes"`
		} `json:"finish"`
		Added []struct {
			Name string `json:"name"`
		} `json:"added"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("report is not JSON: %v", err)
	}

	if report.Finish.Minutes != 24*60 || len(report.Added) != 1 || report.Added[0].Name != "Dewatering" {
		t.Errorf("report = %+v, want the finish a day later and Dewatering added", report)
	}
	if counts[events.CompareRun] != 1 {
		t.Errorf("compare_run counted %d times, want 1", counts[events.CompareRun])
	}
}

func TestRefusesAComparisonWithOnePlan(t *testing.T) {
	counts := tally{}

	rec := comparePlans(t, newCountingHandler(t, 100, counts), []byte(`{"previous":{"contract_version":1}}`))

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), MsgNotAComparison) {
		t.Errorf("got %d %s, want 400 asking for both plans", rec.Code, rec.Body.String())
	}
	if counts[events.CompareRun] != 0 {
		t.Error("a refused comparison was counted")
	}
}

func TestRefusesAComparisonWhenOneSideIsNotAPlan(t *testing.T) {
	body := []byte(`{"previous":{"contract_version":1,"tasks":[],"relations":[]},"current":{"contract_version":2}}`)

	rec := comparePlans(t, newHandler(t, 100), body)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), MsgNotAPlan) {
		t.Errorf("got %d %s, want 400 saying it is not a plan", rec.Code, rec.Body.String())
	}
}

func TestRefusesAnOversizedComparison(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/compare", strings.NewReader("{}"))
	r.ContentLength = MaxComparisonBytes + 1
	r = clientip.SetContext(r, "203.0.113.5")

	rec := httptest.NewRecorder()
	newHandler(t, 100).Compare(rec, r)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d, want 413", rec.Code)
	}
}

func TestComparisonsAreRateLimited(t *testing.T) {
	h := newHandler(t, 1)
	body := versions(t)

	if rec := comparePlans(t, h, body); rec.Code != http.StatusOK {
		t.Fatalf("first comparison got %d", rec.Code)
	}
	if rec := comparePlans(t, h, body); rec.Code != http.StatusTooManyRequests {
		t.Errorf("second comparison got %d, want 429", rec.Code)
	}
}
