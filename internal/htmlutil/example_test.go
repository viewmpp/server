package htmlutil

import (
	"regexp"
	"server/internal/assert"
	"server/internal/examples"
	"strings"
	"testing"
)

var (
	tagPattern  = regexp.MustCompile(`(?s)<[^>]*>`)
	gapPattern  = regexp.MustCompile(`\s+`)
	headPattern = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	navPara     = regexp.MustCompile(`(?s)<p class="sample__more">.*?</p>`)
	prosePart   = regexp.MustCompile(`(?s)<section class="prose">(.*?)</section>`)
	samplePart  = regexp.MustCompile(`(?s)<header class="sample__head[^>]*>(.*?)</header>.*?<section class="sample[^>]*>(.*?)</section>`)
)

func sampleProse(t *testing.T, out string) string {
	t.Helper()

	found := samplePart.FindStringSubmatch(navPara.ReplaceAllString(out, ""))
	if found == nil {
		t.Fatal("the page carries neither a sample header nor a sample section")
	}

	return plainText(found[1] + " " + found[2])
}

func plainText(out string) string {
	return strings.TrimSpace(gapPattern.ReplaceAllString(tagPattern.ReplaceAllString(out, " "), " "))
}

func longSentences(text string) []string {
	var out []string

	for _, sentence := range strings.Split(text, ". ") {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) >= 60 {
			out = append(out, sentence)
		}
	}

	return out
}

func renderExample(t *testing.T, e examples.Example) string {
	t.Helper()

	pages, err := NewPages()
	assert.NilError(t, err)

	return renderPage(t, pages.Example, Page{
		Slug:         "/example/" + e.Name,
		ExampleName:  e.Name,
		ExampleLabel: e.Label,
		FileName:     e.FileName,
		Description:  e.Description(),
		Form:         e,
		Public:       true,
	})
}

func TestExamplePagesDoNotRepeatTheLandingPage(t *testing.T) {
	withBaseURL(t)

	pages, err := NewPages()
	assert.NilError(t, err)

	landing := prosePart.FindStringSubmatch(renderPage(t, pages.App, Page{
		Slug: "/", Description: "x", Public: true, Form: examples.All(),
	}))
	if landing == nil {
		t.Fatal("the landing page has no prose section to compare against")
	}

	home := longSentences(plainText(landing[1]))

	if len(home) < 10 {
		t.Fatalf("only %d long sentences found on the landing page: the comparison would prove nothing", len(home))
	}

	for _, e := range examples.All() {
		t.Run(e.Name, func(t *testing.T) {
			text := plainText(renderExample(t, e))

			for _, sentence := range home {
				if strings.Contains(text, sentence) {
					t.Fatalf("the landing page's prose is served here as well, starting at:\n  %.90s...", sentence)
				}
			}
		})
	}
}

func TestEachExamplePageDescribesItsOwnPlan(t *testing.T) {
	withBaseURL(t)

	for _, e := range examples.All() {
		t.Run(e.Name, func(t *testing.T) {
			out := renderExample(t, e)
			text := plainText(out)

			if !strings.Contains(text, plainText(e.Lead)) {
				t.Error("the lead is missing")
			}

			for i, paragraph := range e.Story {
				if !strings.Contains(text, plainText(paragraph)) {
					t.Errorf("story paragraph %d is missing", i+1)
				}
			}

			stats := e.Stats()
			if stats.Tasks == 0 || stats.Relations == 0 {
				t.Fatal("the counts came out empty, so the facts block says nothing")
			}

			heads := headPattern.FindAllStringSubmatch(out, -1)
			if len(heads) != 1 {
				t.Errorf("the page carries %d h1 elements, want exactly one", len(heads))
			}

			if !strings.Contains(out, `class="foot__nav"`) {
				t.Error("no footer: the page is a dead end for anything following links")
			}

			if !strings.Contains(out, `href="`+e.Format().Path+`"`) {
				t.Error("no link to the page explaining the format this plan was read from")
			}
		})
	}
}

func TestExamplePagesDifferFromEachOther(t *testing.T) {
	withBaseURL(t)

	all := examples.All()

	for i, a := range all {
		for _, b := range all[i+1:] {
			t.Run(a.Name+" vs "+b.Name, func(t *testing.T) {
				second := sampleProse(t, renderExample(t, b))

				for _, sentence := range longSentences(sampleProse(t, renderExample(t, a))) {
					if strings.Contains(second, sentence) {
						t.Fatalf("both pages carry the same sentence:\n  %.90s...", sentence)
					}
				}
			})
		}
	}
}
