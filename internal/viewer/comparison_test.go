package viewer

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"testing"
)

func TestTheComparisonPageShowsARealReport(t *testing.T) {
	body := visit(t, newTestHandler(t).ComparisonPage, comparisonSlug)

	for _, want := range []string{
		"12 Jun 2026", "19 Jun 2026", "7 days later",
		"Added: Asbestos re-inspection", "Removed: Sprinkler alterations",
		`id="pair-previous"`, `id="pair-current"`, "compare-page.js",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page is missing %q", want)
		}
	}
	if strings.Contains(body, "/static/js/app.js") {
		t.Error("the page loads the single-file viewer; its drop zone is not on this page")
	}
}

func TestTheSamplePairReachesTheBrowserIntact(t *testing.T) {
	body := visit(t, newTestHandler(t).ComparisonPage, comparisonSlug)

	found := regexp.MustCompile(`data-compare-example="([^"]*)"`).FindStringSubmatch(body)
	if found == nil {
		t.Fatal("no sample pair on the page")
	}

	var pair struct {
		Previous     map[string]any `json:"previous"`
		Current      map[string]any `json:"current"`
		PreviousName string         `json:"previous_name"`
		CurrentName  string         `json:"current_name"`
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(found[1])), &pair); err != nil {
		t.Fatalf("the sample pair is not JSON once the browser reads the attribute: %v", err)
	}
	if pair.Previous["tasks"] == nil || pair.Current["tasks"] == nil || !strings.HasSuffix(pair.PreviousName, "week 1.mpp") {
		t.Errorf("pair = %+v", pair.PreviousName)
	}
}
