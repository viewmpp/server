package viewer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"server/internal/events"
	"testing"
)

func signal(h *Handler, name string) int {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/signals?name="+url.QueryEscape(name), nil)
	w := httptest.NewRecorder()

	h.Signal(w, r)

	return w.Code
}

func TestTheViewerCanSendTheSignalsItKnows(t *testing.T) {
	known := []string{
		"wish_pdf:owner",
		"wish_pdf:shared",
		"wish_pdf:file",
		"wish_pdf:example",
		"wish_xlsx_report:owner",
		"wish_xlsx_report:shared",
		"wish_xlsx_report:file",
		"wish_xlsx_report:example",
		"export_asked",
		"export_purpose:share",
		"export_purpose:import",
		"export_purpose:analyze",
		"export_purpose:print",
		"export_purpose:other",
		"compare_open:owner",
		"compare_open:shared",
		"compare_open:file",
		"compare_open:example",
		"compare_from:file",
		"compare_from:project",
	}

	for _, name := range known {
		t.Run(name, func(t *testing.T) {
			counts := tally{}

			if code := signal(newCountingHandler(t, counts), name); code != http.StatusNoContent {
				t.Fatalf("got %d, want 204", code)
			}
			if counts[events.Name(name)] != 1 {
				t.Errorf("%s counted %d times, want 1 (counts %v)", name, counts[events.Name(name)], counts)
			}
		})
	}
}

func TestASignalOutsideTheListIsNotCounted(t *testing.T) {
	unknown := []string{
		"",
		"wish_pdf",
		"wish_pdf:",
		"wish_pdf:someone",
		"export_purpose:anything",
		"upload_ok",
		"share_open",
		"export_asked:owner",
		"WISH_PDF:OWNER",
		"compare_open",
		"compare_from:elsewhere",
		"compare_run",
		"upload_for_compare",
	}

	for _, name := range unknown {
		t.Run(name, func(t *testing.T) {
			counts := tally{}

			if code := signal(newCountingHandler(t, counts), name); code != http.StatusNotFound {
				t.Fatalf("got %d, want 404", code)
			}
			if len(counts) != 0 {
				t.Errorf("%q was counted: %v", name, counts)
			}
		})
	}
}
