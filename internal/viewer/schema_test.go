package viewer

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"server/internal/assert"
	"server/internal/examples"
	"server/internal/htmlutil"
	"server/internal/landing"
	"server/internal/session"
	"server/internal/user"
	"testing"
	"time"
)

const baseURL = "https://viewmpp.com"

var schemaPattern = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()

	htmlutil.SetBaseURL(baseURL)

	templates, err := htmlutil.NewTemplates()
	assert.NilError(t, err)

	return NewHandler(templates, baseURL, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func visit(t *testing.T, h http.HandlerFunc, path string) string {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	store := session.NewStore(nil, time.Hour, false, "", slog.New(slog.NewTextHandler(io.Discard, nil)))

	sess, err := store.New(w, r)
	assert.NilError(t, err)

	h(w, user.SetUserContext(session.SetContext(r, sess), user.AnonymousUser))

	assert.Equal(t, w.Code, http.StatusOK)

	return w.Body.String()
}

func graphTypes(t *testing.T, body string) []string {
	t.Helper()

	found := schemaPattern.FindStringSubmatch(body)
	if found == nil {
		t.Fatal("the page carries no ld+json block")
	}

	var document struct {
		Graph []map[string]any `json:"@graph"`
	}
	assert.NilError(t, json.Unmarshal([]byte(found[1]), &document))

	out := make([]string, 0, len(document.Graph))
	for _, node := range document.Graph {
		out = append(out, node["@type"].(string))
	}

	return out
}

func publicRoutes(h *Handler) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/":                                 h.Landing,
		"/examples":                         h.ExamplesPage,
		"/open-mpp-file-without-ms-project": h.WithoutProjectPage,
		"/open-mpp-file-on-mac":             h.MacPage,
		"/open-xer-file-without-primavera":  h.XERPage,
		"/open-microsoft-project-xml":       h.XMLPage,
		"/share-a-project-plan":             h.SharePage,
		"/pricing":                          h.PricingPage,
		"/cookies":                          h.CookiesPage,
		"/privacy":                          h.PrivacyPage,
		"/terms":                            h.TermsPage,
	}
}

func TestPublicPagesReachTheBrowserWithTheirMarkup(t *testing.T) {
	h := newTestHandler(t)

	for slug, handler := range publicRoutes(h) {
		t.Run(slug, func(t *testing.T) {
			types := graphTypes(t, visit(t, handler, slug))

			want := "BreadcrumbList"
			if slug == "/" {
				want = "WebSite"
			}

			for _, got := range types {
				if got == want {
					return
				}
			}

			t.Errorf("%s reached the browser without %s, got %v", slug, want, types)
		})
	}
}

func TestNoLandingPageIsLeftUnchecked(t *testing.T) {
	routes := publicRoutes(newTestHandler(t))

	for _, page := range landing.All() {
		if page.Slug == "/mpp-to-excel" {
			continue
		}

		if _, covered := routes[page.Slug]; !covered {
			t.Errorf("%s is in the landing metadata and nothing here proves it carries markup", page.Slug)
		}
	}
}

func TestExamplePagesCarryTheirOwnTrail(t *testing.T) {
	h := newTestHandler(t)

	for _, e := range examples.All() {
		t.Run(e.Name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/example/"+e.Name, nil)
			r.SetPathValue("name", e.Name)

			w := httptest.NewRecorder()

			store := session.NewStore(nil, time.Hour, false, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
			sess, err := store.New(w, r)
			assert.NilError(t, err)

			h.ExamplePage(w, user.SetUserContext(session.SetContext(r, sess), user.AnonymousUser))

			assert.Equal(t, w.Code, http.StatusOK)

			types := graphTypes(t, w.Body.String())

			for _, got := range types {
				if got == "SoftwareApplication" {
					t.Error("an example page is a plan, not a second copy of the product")
				}
			}

			if len(types) != 1 || types[0] != "BreadcrumbList" {
				t.Errorf("example page graph = %v, want a single BreadcrumbList", types)
			}
		})
	}
}
