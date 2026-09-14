package htmlutil

import (
	"encoding/json"
	"html/template"
	"server/internal/examples"
	"server/internal/landing"
	"strings"
)

const (
	siteName      = "View MPP"
	siteAltName   = "MPP viewer"
	schemaContext = "https://schema.org"
	homeCrumb     = "Home"
)

type schemaRef struct {
	ID string `json:"@id"`
}

type schemaImage struct {
	Type   string `json:"@type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type schemaOrganization struct {
	Type string       `json:"@type"`
	ID   string       `json:"@id"`
	Name string       `json:"name"`
	URL  string       `json:"url"`
	Logo *schemaImage `json:"logo,omitempty"`
}

type schemaWebsite struct {
	Type          string    `json:"@type"`
	ID            string    `json:"@id"`
	URL           string    `json:"url"`
	Name          string    `json:"name"`
	AlternateName string    `json:"alternateName"`
	Publisher     schemaRef `json:"publisher"`
}

type schemaOffer struct {
	Type          string `json:"@type"`
	Price         string `json:"price"`
	PriceCurrency string `json:"priceCurrency"`
}

type schemaApplication struct {
	Type                string      `json:"@type"`
	ID                  string      `json:"@id"`
	Name                string      `json:"name"`
	URL                 string      `json:"url"`
	ApplicationCategory string      `json:"applicationCategory"`
	OperatingSystem     string      `json:"operatingSystem"`
	BrowserRequirements string      `json:"browserRequirements"`
	IsAccessibleForFree bool        `json:"isAccessibleForFree"`
	FeatureList         []string    `json:"featureList"`
	Offers              schemaOffer `json:"offers"`
	Publisher           schemaRef   `json:"publisher"`
}

type schemaListItem struct {
	Type     string `json:"@type"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	Item     string `json:"item,omitempty"`
}

type schemaBreadcrumbs struct {
	Type  string           `json:"@type"`
	Items []schemaListItem `json:"itemListElement"`
}

type schemaDocument struct {
	Context string `json:"@context"`
	Graph   []any  `json:"@graph"`
}

type Crumb struct {
	Name string
	URL  string
}

const examplePrefix = "/example/"

func (p Page) Crumbs() []Crumb {
	return p.trail()
}

func (p Page) Schema() template.JS {
	if baseURL == "" || p.Slug == "" {
		return ""
	}

	if strings.HasPrefix(p.Slug, examplePrefix) {
		trail := p.trail()
		if trail == nil {
			return ""
		}
		return encode(breadcrumbs(trail))
	}

	page := landing.BySlug(p.Slug)
	if page.Slug == "" {
		return ""
	}

	if page.Slug == "/" {
		return encode(website(), organization(true), application())
	}

	nodes := make([]any, 0, 3)

	if page.Tool {
		nodes = append(nodes, organization(false), application())
	}

	return encode(append(nodes, breadcrumbs(p.trail()))...)
}

func (p Page) trail() []Crumb {
	if name, isExample := strings.CutPrefix(p.Slug, examplePrefix); isExample {
		e, exists := examples.ByName(name)
		if !exists {
			return nil
		}

		return []Crumb{
			{Name: homeCrumb, URL: "/"},
			{Name: landing.BySlug("/examples").Label, URL: "/examples"},
			{Name: e.Label},
		}
	}

	page := landing.BySlug(p.Slug)
	if page.Slug == "" || page.Slug == "/" {
		return nil
	}

	return []Crumb{
		{Name: homeCrumb, URL: "/"},
		{Name: page.Label},
	}
}

func website() schemaWebsite {
	return schemaWebsite{
		Type:          "WebSite",
		ID:            baseURL + "/#website",
		URL:           baseURL + "/",
		Name:          siteName,
		AlternateName: siteAltName,
		Publisher:     schemaRef{ID: baseURL + "/#organization"},
	}
}

func organization(withLogo bool) schemaOrganization {
	node := schemaOrganization{
		Type: "Organization",
		ID:   baseURL + "/#organization",
		Name: siteName,
		URL:  baseURL + "/",
	}

	if withLogo {
		node.Logo = &schemaImage{
			Type:   "ImageObject",
			URL:    baseURL + "/static/icons/icon-192.png",
			Width:  192,
			Height: 192,
		}
	}

	return node
}

func application() schemaApplication {
	return schemaApplication{
		Type:                "SoftwareApplication",
		ID:                  baseURL + "/#app",
		Name:                siteName,
		URL:                 baseURL + "/",
		ApplicationCategory: "BusinessApplication",
		OperatingSystem:     "Any (web browser)",
		BrowserRequirements: "Requires JavaScript",
		IsAccessibleForFree: true,
		FeatureList: []string{
			"Opens .mpp, Microsoft Project .xml, .mpx, .mpd and Primavera .xer files",
			"Interactive Gantt chart with dependencies, critical path and non-working days",
			"Task table with search and collapsible hierarchy",
			"Task detail panel with dates, duration, progress, resources and notes",
			"Export to Excel (.xlsx)",
			"Read-only share links",
		},
		Offers: schemaOffer{
			Type:          "Offer",
			Price:         "0",
			PriceCurrency: "USD",
		},
		Publisher: schemaRef{ID: baseURL + "/#organization"},
	}
}

func breadcrumbs(trail []Crumb) schemaBreadcrumbs {
	items := make([]schemaListItem, 0, len(trail))

	for i, c := range trail {
		item := schemaListItem{
			Type:     "ListItem",
			Position: i + 1,
			Name:     c.Name,
		}

		if c.URL != "" {
			item.Item = baseURL + c.URL
		}

		items = append(items, item)
	}

	return schemaBreadcrumbs{Type: "BreadcrumbList", Items: items}
}

func encode(nodes ...any) template.JS {
	body, err := json.Marshal(schemaDocument{Context: schemaContext, Graph: nodes})
	if err != nil {
		return ""
	}

	return template.JS(body)
}
