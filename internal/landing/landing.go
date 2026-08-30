package landing

type Page struct {
	Slug        string
	Label       string
	Group       Group
	Description string
}

type Group string

const (
	GroupConvert Group = "convert"
	GroupView    Group = "view"
	GroupFormat  Group = "format"
	GroupProduct Group = "product"
)

var groupOrder = []Group{GroupConvert, GroupView, GroupFormat, GroupProduct}

var pages = []Page{
	{
		Slug:        "/",
		Description: "Open an .mpp or Project .xml file in your browser and read the Gantt chart at once - tasks, dates, dependencies, critical path. No install, no signup.",
	},
	{
		Slug:        "/examples",
		Label:       "Examples",
		Group:       GroupProduct,
		Description: "Open a sample MS Project plan in your browser - Gantt chart, task table and dependencies, with no file of your own and no signup.",
	},
	{
		Slug:        "/mpp-to-excel",
		Group:       GroupConvert,
		Label:       "MPP to Excel",
		Description: "Convert an MS Project .mpp file to an Excel .xlsx spreadsheet in your browser - tasks, dates, durations and predecessors, with no install and no signup.",
	},
	{
		Slug:        "/pricing",
		Group:       GroupProduct,
		Label:       "Pricing",
		Description: "What the free tier gives you and what Pro adds: unlimited saved plans and share links, password-protected links, 50 MB uploads. Reading stays free.",
	},
	{
		Slug:        "/share-a-project-plan",
		Group:       GroupProduct,
		Label:       "Share a plan",
		Description: "Your team has no Microsoft Project licences and a PDF freezes the schedule. Send a link instead - the plan stays readable, foldable and searchable.",
	},
	{
		Slug:        "/open-mpp-file-without-ms-project",
		Group:       GroupView,
		Label:       "Open .mpp without MS Project",
		Description: "Sent an .mpp file and have no Microsoft Project? What is inside that file, why a text editor cannot show it, and how to read it in your browser.",
	},
	{
		Slug:        "/mpp-viewer-mac",
		Group:       GroupView,
		Label:       "Open .mpp on a Mac",
		Description: "Microsoft Project has no macOS version at all. What that leaves you with, what a virtual machine really costs, and how to read .mpp on a Mac instead.",
	},
	{
		Slug:        "/privacy",
		Description: "What MPP Viewer stores, what it does not, and how long anything is kept. Uploaded files are never written to disk.",
	},
	{
		Slug:        "/terms",
		Description: "The terms for using MPP Viewer: what the service does, what you may upload, how sharing works and what the limits are.",
	},
}

func All() []Page {
	return pages
}

func Footer() []Page {
	out := make([]Page, 0, len(pages))

	for _, group := range groupOrder {
		for _, p := range pages {
			if p.Group == group && p.Label != "" {
				out = append(out, p)
			}
		}
	}

	return out
}

func BySlug(slug string) Page {
	for _, p := range pages {
		if p.Slug == slug {
			return p
		}
	}
	return Page{}
}
