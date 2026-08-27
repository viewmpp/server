package htmlutil

import (
	"bytes"
	"server/internal/assert"
	"server/ui"
	"strings"
	"testing"
)

func TestIconsAreEmbedded(t *testing.T) {
	linked := []string{
		"static/icons/favicon.ico",
		"static/icons/icon-192.png",
		"static/icons/apple-touch-icon.png",
		"static/icons/og.png",
	}

	for _, name := range linked {
		blob, err := ui.Files.ReadFile(name)
		if err != nil {
			t.Errorf("%s is linked from every page but is not in the binary: %v", name, err)
			continue
		}

		if len(blob) == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestSocialImageIsAbsolute(t *testing.T) {
	SetBaseURL("https://viewmpp.com/")
	t.Cleanup(func() { SetBaseURL("") })

	got := Page{Version: "abc"}.OGImage()

	assert.Equal(t, got, "https://viewmpp.com/static/icons/og.png?v=abc")
}

func TestSocialImageIsOmittedWithoutABaseURL(t *testing.T) {
	SetBaseURL("")

	if got := (Page{Version: "abc"}).OGImage(); got != "" {
		t.Fatalf("OGImage = %q, want empty: a relative path is ignored by every link preview", got)
	}
}

func TestPagesCarryTheIconsAndTheSocialImage(t *testing.T) {
	SetBaseURL("https://viewmpp.com")
	t.Cleanup(func() { SetBaseURL("") })

	pages, err := NewPages()
	assert.NilError(t, err)

	want := []string{
		`<link rel="icon" href="/static/icons/favicon.ico`,
		`<link rel="apple-touch-icon" href="/static/icons/apple-touch-icon.png`,
		`<meta property="og:image" content="https://viewmpp.com/static/icons/og.png`,
		`<meta name="twitter:card" content="summary_large_image">`,
	}

	var buf bytes.Buffer
	page := Page{Description: "A plan in the browser", Form: stubForm{}}

	if err := pages.Pricing.ExecuteTemplate(&buf, "base", page); err != nil {
		t.Fatal(err)
	}

	for _, tag := range want {
		if !strings.Contains(buf.String(), tag) {
			t.Errorf("rendered page is missing %s", tag)
		}
	}
}
