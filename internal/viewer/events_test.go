package viewer

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/assert"
	"server/internal/events"
	"server/internal/examples"
	"server/internal/session"
	"server/internal/user"
	"testing"
	"time"
)

func visitFrom(t *testing.T, h http.HandlerFunc, path, referer string) {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	if referer != "" {
		r.Header.Set("Referer", referer)
	}
	w := httptest.NewRecorder()

	sess, err := session.NewStore(nil, time.Hour, false, "", slog.New(slog.NewTextHandler(io.Discard, nil))).New(w, r)
	assert.NilError(t, err)

	h(w, user.SetUserContext(session.SetContext(r, sess), user.AnonymousUser))

	assert.Equal(t, w.Code, http.StatusOK)
}

func TestTheSharePageCountsVisitorsArrivingFromAPlan(t *testing.T) {
	tests := map[string]struct {
		referer string
		want    int
	}{
		"from a shared plan":           {baseURL + "/p/L5uTFtII7Ef_uvsK", 1},
		"from the home page":           {baseURL + "/", 0},
		"from a look-alike elsewhere":  {"https://elsewhere.example/p/abc", 0},
		"with no referer":              {"", 0},
		"from a path that only starts": {baseURL + "/pricing", 0},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			counts := tally{}
			h := newCountingHandler(t, counts)

			visitFrom(t, h.SharePage, "/share-a-project-plan", tt.referer)

			if got := counts[events.SharePageFromLink]; got != tt.want {
				t.Errorf("share_page_from_link counted %d times, want %d", got, tt.want)
			}
		})
	}
}

func TestOpeningAnExampleIsCounted(t *testing.T) {
	counts := tally{}
	h := newCountingHandler(t, counts)

	e := examples.All()[0]

	r := httptest.NewRequest(http.MethodGet, "/api/v1/examples/"+e.Name, nil)
	r.SetPathValue("name", e.Name)
	w := httptest.NewRecorder()

	h.ExampleContract(w, r)

	assert.Equal(t, w.Code, http.StatusOK)

	if counts[events.ExampleOpen] != 1 {
		t.Errorf("example_open counted %d times, want 1", counts[events.ExampleOpen])
	}
}
