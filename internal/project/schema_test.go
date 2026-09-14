package project

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"server/internal/assert"
	"server/internal/htmlutil"
	"server/internal/session"
	"server/internal/user"
	"testing"
	"time"
)

var schemaPattern = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

func TestTheConvertPageDeclaresTheApplication(t *testing.T) {
	const baseURL = "https://viewmpp.com"

	htmlutil.SetBaseURL(baseURL)

	templates, err := htmlutil.NewTemplates()
	assert.NilError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := NewHandler(NewStore(nil), baseURL, nil, 10, templates, logger)

	r := httptest.NewRequest(http.MethodGet, "/mpp-to-excel", nil)
	w := httptest.NewRecorder()

	sess, err := session.NewStore(nil, time.Hour, false, "", logger).New(w, r)
	assert.NilError(t, err)

	h.ConvertPage(w, user.SetUserContext(session.SetContext(r, sess), user.AnonymousUser))

	assert.Equal(t, w.Code, http.StatusOK)

	found := schemaPattern.FindStringSubmatch(w.Body.String())
	if found == nil {
		t.Fatal("the convert page carries no ld+json block")
	}

	var document struct {
		Graph []map[string]any `json:"@graph"`
	}
	assert.NilError(t, json.Unmarshal([]byte(found[1]), &document))

	seen := map[string]bool{}
	for _, node := range document.Graph {
		seen[node["@type"].(string)] = true
	}

	for _, want := range []string{"SoftwareApplication", "BreadcrumbList"} {
		if !seen[want] {
			t.Errorf("%s is missing: the page opens a file and sits one step below the home page", want)
		}
	}
}
