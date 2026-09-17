package htmlutil

import (
	"html/template"
	"regexp"
	"server/internal/examples"
	"server/internal/landing"
	"strings"
	"testing"
)

const (
	minAnswerWords = 40
	maxAnswerWords = 70
)

var (
	answerPara = regexp.MustCompile(`(?s)<p class="prose__answer">(.*?)</p>`)
	wordish    = regexp.MustCompile(`\w`)
)

func countWords(text string) int {
	n := 0

	for _, word := range strings.Fields(text) {
		if wordish.MatchString(word) {
			n++
		}
	}

	return n
}

func renderCluster(t *testing.T, tmpl *template.Template, slug string) string {
	t.Helper()

	page := Page{Slug: slug, Description: "x", Public: true, Examples: examples.All()}
	if slug == "/" || slug == "/examples" {
		page.Form = examples.All()
	}

	return renderPage(t, tmpl, page)
}

func TestClusterPagesOpenWithAStandaloneAnswer(t *testing.T) {
	withBaseURL(t)

	for slug, tmpl := range indexable(t) {
		if landing.BySlug(slug).Group == landing.GroupLegal {
			continue
		}

		if slug == "/pricing" {
			continue
		}

		t.Run(slug, func(t *testing.T) {
			out := renderCluster(t, tmpl, slug)

			found := answerPara.FindAllStringSubmatch(out, -1)
			if len(found) != 1 {
				t.Fatalf("the page carries %d answer paragraphs, want exactly one", len(found))
			}

			body := prosePart.FindStringSubmatch(out)
			if body == nil {
				t.Fatal("no prose section")
			}

			if !strings.HasPrefix(strings.TrimSpace(body[1]), `<p class="prose__answer">`) {
				t.Error("the answer does not open the prose: a reader meets a heading first, and the paragraph stops being the page's answer")
			}

			if n := countWords(plainText(found[0][1])); n < minAnswerWords || n > maxAnswerWords {
				t.Errorf("the answer is %d words, want %d to %d: shorter answers nothing, longer stops being quotable\n  %s",
					n, minAnswerWords, maxAnswerWords, plainText(found[0][1]))
			}
		})
	}
}
