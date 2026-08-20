package landing

type Page struct {
	Slug        string
	Label       string
	Description string
}

var pages = []Page{
	{
		Slug:        "/",
		Description: "Open an MS Project .mpp or .xml file in your browser and read the Gantt chart straight away - tasks, dates, dependencies and the critical path, with no install and no signup.",
	},
	{
		Slug:        "/examples",
		Description: "Open a sample MS Project plan in your browser - Gantt chart, task table and dependencies, with no file of your own and no signup.",
	},
	{
		Slug:        "/mpp-to-excel",
		Label:       "MPP to Excel",
		Description: "Convert an MS Project .mpp file to an Excel .xlsx spreadsheet in your browser - tasks, dates, durations and predecessors, with no install and no signup.",
	},
	{
		Slug:        "/pricing",
		Label:       "Pricing",
		Description: "What the free tier gives you, what Pro adds, and why Pro costs nothing during early access. Reading and converting a plan is free and needs no account.",
	},
	{
		Slug:        "/open-mpp-file-without-ms-project",
		Label:       "Open .mpp without MS Project",
		Description: "Someone sent you an .mpp file and you have no Microsoft Project. Here is what is inside that file, why a text editor cannot show it, and how to read it in a browser instead.",
	},
	{
		Slug:        "/mpp-viewer-mac",
		Label:       "Open .mpp on a Mac",
		Description: "Microsoft Project has no macOS version at all. Here is what that leaves you with, what a virtual machine really costs, and how to read an .mpp file on a Mac in the browser instead.",
	},
}

func All() []Page {
	return pages
}

func Footer() []Page {
	out := make([]Page, 0, len(pages))
	for _, p := range pages {
		if p.Label != "" {
			out = append(out, p)
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
