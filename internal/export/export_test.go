package export

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/clientip"
	"server/internal/contract"
	"server/internal/events"
	"server/internal/fixtures"
	"server/internal/ratelimit"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

type tally map[events.Name]int

func (t tally) Count(name events.Name) {
	t[name]++
}

func newHandler(t *testing.T, limit int) *Handler {
	t.Helper()

	return newCountingHandler(t, limit, tally{})
}

func newCountingHandler(t *testing.T, limit int, counts tally) *Handler {
	t.Helper()

	limiter := ratelimit.New(limit, time.Minute)
	t.Cleanup(limiter.Close)

	return NewHandler(limiter, counts, slog.New(slog.DiscardHandler))
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

func planWithRelations(t *testing.T, relations []contract.Relation) []byte {
	t.Helper()

	name := "Handover"
	tasks := make([]contract.Task, 0, len(relations)+1)
	tasks = append(tasks, contract.Task{ID: 1, Name: &name})
	for _, rel := range relations {
		other := rel.PredecessorID
		if other == 1 {
			other = rel.SuccessorID
		}
		tasks = append(tasks, contract.Task{ID: other, Name: &name})
	}

	raw, err := json.Marshal(contract.Contract{Version: contract.Version, Tasks: tasks, Relations: relations})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestExportsATaskWithManySuccessors(t *testing.T) {
	relations := make([]contract.Relation, 300)
	for i := range relations {
		relations[i] = contract.Relation{ID: int64(i + 1), PredecessorID: 1, SuccessorID: int64(i + 2), Type: "FINISH_START"}
	}

	rec := post(t, newHandler(t, 100), planWithRelations(t, relations), "plan.mpp")

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestSaysWhenAPlanIsTooLargeRatherThanInvalid(t *testing.T) {
	relations := make([]contract.Relation, contract.MaxPredecessorsPerTask+1)
	for i := range relations {
		relations[i] = contract.Relation{ID: int64(i + 1), PredecessorID: int64(i + 2), SuccessorID: 1, Type: "FINISH_START"}
	}

	rec := post(t, newHandler(t, 100), planWithRelations(t, relations), "plan.mpp")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}

	var message string
	if err := json.Unmarshal(rec.Body.Bytes(), &message); err != nil {
		t.Fatalf("body is not a json string: %v", err)
	}
	if message != MsgTooLargeToConvert {
		t.Errorf("got %q, want %q", message, MsgTooLargeToConvert)
	}
}

func TestSaysWhenABodyIsNotAPlan(t *testing.T) {
	rec := post(t, newHandler(t, 100), []byte(`{"contract_version":1,"surprise":true}`), "x.mpp")

	var message string
	if err := json.Unmarshal(rec.Body.Bytes(), &message); err != nil {
		t.Fatalf("body is not a json string: %v", err)
	}
	if message != MsgNotAPlan {
		t.Errorf("got %q, want %q", message, MsgNotAPlan)
	}
}

func TestCountsAnExportOnlyWhenTheWorkbookIsSent(t *testing.T) {
	counts := tally{}
	h := newCountingHandler(t, 100, counts)

	post(t, h, fixtures.All()["mpp8.json"], "plan.mpp")
	post(t, h, []byte(`hello`), "x.mpp")

	if counts[events.ExportXLSX] != 1 {
		t.Errorf("export_xlsx counted %d times, want 1", counts[events.ExportXLSX])
	}
}

func TestCountsAPlanOverTheLimit(t *testing.T) {
	relations := make([]contract.Relation, contract.MaxPredecessorsPerTask+1)
	for i := range relations {
		relations[i] = contract.Relation{ID: int64(i + 1), PredecessorID: int64(i + 2), SuccessorID: 1, Type: "FINISH_START"}
	}

	counts := tally{}
	post(t, newCountingHandler(t, 100, counts), planWithRelations(t, relations), "plan.mpp")

	if counts[events.ExportTooLarge] != 1 || counts[events.ExportXLSX] != 0 {
		t.Errorf("counts = %v, want one export_too_large and no export_xlsx", counts)
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

func TestALookaheadWorkbookIsCountedApartFromAFullExport(t *testing.T) {
	counts := tally{}
	h := newCountingHandler(t, 100, counts)

	rec := post(t, h, fixtures.All()["mpp8.json"], "plan.mpp&lookahead_from=2014-10-27&lookahead_weeks=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", rec.Code, rec.Body.String())
	}

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("result is not a workbook: %v", err)
	}
	defer func() { _ = f.Close() }()
	if idx, _ := f.GetSheetIndex("Lookahead"); idx == -1 {
		t.Error("no Lookahead sheet in a lookahead export")
	}

	if counts[events.ExportLookahead] != 1 || counts[events.ExportXLSX] != 0 {
		t.Errorf("export_lookahead = %d, export_xlsx = %d: a lookahead is not a full export", counts[events.ExportLookahead], counts[events.ExportXLSX])
	}
}

func TestAMalformedLookaheadWindowIsRefused(t *testing.T) {
	counts := tally{}
	h := newCountingHandler(t, 100, counts)

	for _, query := range []string{"&lookahead_from=yesterday&lookahead_weeks=3", "&lookahead_from=2014-10-27&lookahead_weeks=52", "&lookahead_weeks=3"} {
		if rec := post(t, h, fixtures.All()["mpp8.json"], "plan.mpp"+query); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", query, rec.Code)
		}
	}
	if len(counts) != 0 {
		t.Errorf("refused exports were counted: %v", counts)
	}
}
