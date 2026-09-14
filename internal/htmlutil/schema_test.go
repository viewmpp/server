package htmlutil

import (
	"encoding/json"
	"regexp"
	"server/internal/assert"
	"server/internal/examples"
	"server/internal/landing"
	"strings"
	"testing"
)

const testBaseURL = "https://viewmpp.com"

var schemaPattern = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

func withBaseURL(t *testing.T) {
	t.Helper()

	previous := baseURL
	SetBaseURL(testBaseURL)
	t.Cleanup(func() { SetBaseURL(previous) })
}

func graphOf(t *testing.T, body string) []map[string]any {
	t.Helper()

	var document struct {
		Context string           `json:"@context"`
		Graph   []map[string]any `json:"@graph"`
	}

	assert.NilError(t, json.Unmarshal([]byte(body), &document))
	assert.Equal(t, document.Context, "https://schema.org")

	return document.Graph
}

func typesIn(graph []map[string]any) []string {
	out := make([]string, 0, len(graph))
	for _, node := range graph {
		out = append(out, node["@type"].(string))
	}
	return out
}

func has(types []string, want string) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

func TestSchemaIsSilentWithoutABaseURL(t *testing.T) {
	previous := baseURL
	SetBaseURL("")
	t.Cleanup(func() { SetBaseURL(previous) })

	if Schema("/") != "" {
		t.Error("absolute @id values are impossible without a base url, so no markup may be written")
	}
}

func TestSchemaIgnoresPagesOutsideTheLandingSet(t *testing.T) {
	withBaseURL(t)

	for _, slug := range []string{"/p/abc123", "/projects", "/nothing-here"} {
		if Schema(slug) != "" {
			t.Errorf("%s is not a landing page and must carry no markup", slug)
		}
	}
}

func TestEveryLandingPageCarriesValidMarkup(t *testing.T) {
	withBaseURL(t)

	for _, page := range landing.All() {
		t.Run(page.Slug, func(t *testing.T) {
			body := string(Schema(page.Slug))
			if body == "" {
				t.Fatal("no markup: the page is in the landing set and is indexable")
			}

			graph := graphOf(t, body)
			if len(graph) == 0 {
				t.Fatal("empty @graph")
			}
		})
	}
}

func TestHomeCarriesTheSiteWideEntities(t *testing.T) {
	withBaseURL(t)

	graph := graphOf(t, string(Schema("/")))
	types := typesIn(graph)

	for _, want := range []string{"WebSite", "Organization", "SoftwareApplication"} {
		if !has(types, want) {
			t.Errorf("%s is missing from the home page graph, got %v", want, types)
		}
	}

	if has(types, "BreadcrumbList") {
		t.Error("the home page is the root of the trail and has no breadcrumbs of its own")
	}
}

func TestToolPagesDeclareTheApplicationAndPlainPagesDoNot(t *testing.T) {
	withBaseURL(t)

	for _, page := range landing.All() {
		if page.Slug == "/" {
			continue
		}

		t.Run(page.Slug, func(t *testing.T) {
			types := typesIn(graphOf(t, string(Schema(page.Slug))))

			if !has(types, "BreadcrumbList") {
				t.Errorf("no breadcrumbs, got %v", types)
			}

			if got := has(types, "SoftwareApplication"); got != page.Tool {
				t.Errorf("SoftwareApplication present = %v, want %v: the claim belongs on pages that actually open a file", got, page.Tool)
			}
		})
	}
}

func TestTheApplicationIsOneEntityAcrossPages(t *testing.T) {
	withBaseURL(t)

	id := func(slug string) string {
		for _, node := range graphOf(t, string(Schema(slug))) {
			if node["@type"] == "SoftwareApplication" {
				return node["@id"].(string)
			}
		}
		t.Fatalf("%s carries no SoftwareApplication", slug)
		return ""
	}

	home := id("/")

	for _, page := range landing.All() {
		if !page.Tool || page.Slug == "/" {
			continue
		}

		if got := id(page.Slug); got != home {
			t.Errorf("%s declares @id %q, home declares %q: a different id makes it a second product", page.Slug, got, home)
		}
	}
}

func TestTheApplicationIsFreeToUse(t *testing.T) {
	withBaseURL(t)

	for _, node := range graphOf(t, string(Schema("/"))) {
		if node["@type"] != "SoftwareApplication" {
			continue
		}

		offer, ok := node["offers"].(map[string]any)
		if !ok {
			t.Fatal("no offer: a price is what tells an assistant the tool costs nothing")
		}

		assert.Equal(t, offer["price"].(string), "0")
		assert.Equal(t, node["isAccessibleForFree"].(bool), true)

		return
	}

	t.Fatal("no SoftwareApplication on the home page")
}

func TestBreadcrumbsEndOnThePageItself(t *testing.T) {
	withBaseURL(t)

	trail := breadcrumbsIn(t, string(Schema("/open-mpp-file-on-mac")))

	if len(trail) != 2 {
		t.Fatalf("trail has %d steps, want 2: home then the page", len(trail))
	}

	assert.Equal(t, trail[0]["name"].(string), "Home")
	assert.Equal(t, trail[0]["item"].(string), testBaseURL+"/")
	assert.Equal(t, trail[1]["name"].(string), landing.BySlug("/open-mpp-file-on-mac").Label)

	if _, present := trail[1]["item"]; present {
		t.Error("the last step is the current page and carries no link")
	}
}

func TestExamplePagesSitUnderTheExamplesIndex(t *testing.T) {
	withBaseURL(t)

	for _, e := range examples.All() {
		t.Run(e.Name, func(t *testing.T) {
			trail := breadcrumbsIn(t, string(ExampleSchema(e)))

			if len(trail) != 3 {
				t.Fatalf("trail has %d steps, want 3: home, examples, the plan", len(trail))
			}

			assert.Equal(t, trail[1]["item"].(string), testBaseURL+"/examples")
			assert.Equal(t, trail[2]["name"].(string), e.Label)

			for i, step := range trail {
				assert.Equal(t, int(step["position"].(float64)), i+1)
			}
		})
	}
}

func breadcrumbsIn(t *testing.T, body string) []map[string]any {
	t.Helper()

	for _, node := range graphOf(t, body) {
		if node["@type"] != "BreadcrumbList" {
			continue
		}

		items := node["itemListElement"].([]any)

		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, item.(map[string]any))
		}

		return out
	}

	t.Fatal("no BreadcrumbList in the graph")
	return nil
}

func TestMarkupSurvivesTheTemplate(t *testing.T) {
	withBaseURL(t)

	out := render(t, Page{Description: "x", Public: true, Schema: Schema("/")})

	found := schemaPattern.FindStringSubmatch(out)
	if found == nil {
		t.Fatal("the rendered page carries no ld+json block")
	}

	graph := graphOf(t, found[1])
	if len(graph) != 3 {
		t.Fatalf("the graph lost nodes on the way through the template: %v", typesIn(graph))
	}

	if strings.Contains(found[1], "</") {
		t.Error("an unescaped closing tag inside the script would end the block early")
	}
}

func TestPagesWithoutMarkupWriteNoEmptyBlock(t *testing.T) {
	if schemaPattern.MatchString(render(t, Page{})) {
		t.Error("a page with no schema emitted an empty ld+json block")
	}
}
