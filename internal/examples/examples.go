package examples

import (
	"embed"
	"encoding/json"
	"fmt"
	"server/internal/contract"
	"strings"
	"sync"
	"time"
)

//go:embed *.json
var files embed.FS

type Example struct {
	Name     string
	Label    string
	Note     string
	FileName string
	Lead     string
	Story    []string
	anchor   time.Time
}

type Stats struct {
	Tasks       int
	Summaries   int
	Milestones  int
	Critical    int
	Depth       int
	Resources   int
	Assignments int
	Relations   int
	Exceptions  int
}

type Format struct {
	Path  string
	Label string
}

func anchoredOn(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

var catalogue = []Example{
	{
		Name:     "office-fit-out",
		Label:    "Office fit-out",
		Note:     "62 tasks, a critical path under pressure and four jobs already late",
		FileName: "Fenchurch St - level 4 fit-out.xml",
		Lead:     "Sixty-two tasks fitting out one floor at Fenchurch Street, read from a Project .xml export. The chart below is the plan itself, not a picture of it.",
		Story: []string{
			"The plan runs in five phases - pre-construction, design and procurement, site works, finishes, and commissioning and handover. It ends at practical completion, and the four milestones before it - agreement for lease signed, client sign-off on layout, landlord licence to alter, contractor appointed - are the gates a fit-out actually waits on.",
			"Forty-one of the sixty-two tasks are marked critical, which is high and not an accident: fit-out trades are sequential. Partitions wait on services, finishes wait on partitions, so most of the schedule is one long chain with very little slack beside it. Collapse Site works and the shape is visible on one screen.",
			"Structure, dependencies and calendar are real; the content is invented and no client data appears. Three exceptions sit in the calendar - two bank holidays and a contractor shutdown - and the whole plan shifts with the calendar every week, so a task counts as late against today rather than against the day the file was built.",
		},
		anchor: anchoredOn(2026, time.June, 18),
	},
	{
		Name:     "viaduct",
		Label:    "Виадук (Cyrillic)",
		Note:     "non-Latin names throughout, Russian holidays shaded, a milestone missed",
		FileName: "Виадук через Каменку - этап 2.xml",
		Lead:     "Fifty-three tasks building a viaduct across the Kamenka, with every task name, resource and calendar entry in Cyrillic. Read from a Project .xml export.",
		Story: []string{
			"Six phases - подготовительный период, опоры, пролётное строение, мостовое полотно, подходы и обустройство, сдача. Four milestones mark the points the job is judged on: the site handed to the contractor, traffic moved onto the diversion, every pier concreted, and the bridge brought into service.",
			"This plan exists to break things. Non-Latin text is where cheap readers fail, usually returning rows of question marks, because the older .mpp generations store strings in a way that punishes a careless implementation. Nine resource names, three calendar exceptions and every task name here are Cyrillic; if any of it arrives as garbage, the reader is wrong.",
			"Twenty-seven of the fifty-three tasks are critical - about half, the ordinary shape for structural work where a few chains run in parallel. The longest single bar is подходы и обустройство at ninety-seven days, and it is not on the critical path. Length and criticality are different things, and this plan shows the difference on one screen.",
		},
		anchor: anchoredOn(2026, time.July, 15),
	},
	{
		Name:     "substation",
		Label:    "Substation extension",
		Note:     "read from an .mpx, with a nine-month lead time holding the critical path",
		FileName: "Wraysbury 132kV - bay 4 extension.mpx",
		Lead:     "Fifty tasks extending a 132 kV substation, read from an .mpx - the plain-text format Microsoft dropped a quarter of a century ago and half the industry still exchanges.",
		Story: []string{
			"Six phases - design and approvals, long-lead procurement, civil works, plant installation, testing and commissioning, outage and energisation. Ten milestones, twice as many as the other samples here, because a connection job is a sequence of gates: connection agreement signed, design approved, outage window confirmed, plant delivered to site, civils complete, outage starts, circuit energised, handover to operations.",
			"The critical path runs through procurement, not through site work. Long-lead procurement is a hundred and ten days and it is critical - the longest bar in the plan is a factory lead time, and nothing on site can pull the job forward while it runs. That is usually the answer people are after when they ask why a job cannot be accelerated.",
			"The outline is two levels deep rather than three, and the six calendar exceptions carry no names at all. An .mpx keeps less than a Project .xml does, and the reader shows what is in the file rather than filling the gaps. Structure and dependencies are real; the content is invented.",
		},
		anchor: anchoredOn(2026, time.July, 20),
	},
	{
		Name:     "wms-rollout",
		Label:    "System rollout",
		Note:     "57 tasks across build, data, testing and cutover",
		FileName: "WMS rollout - Tilbury DC.xml",
		Lead:     "Fifty-seven tasks putting a warehouse management system into a distribution centre - build, data, testing, cutover. Read from a Project .xml export.",
		Story: []string{
			"Seven phases - mobilisation, discovery, build and configuration, data, testing, training and readiness, cutover and hypercare. Five milestones: contract signed, solution design signed off, UAT sign-off, the go-live decision, and go live itself.",
			"Thirty-five of the fifty-seven tasks are critical, and the longest bar - build and configuration, fifty-five days - is one of them. This is the shape of plan most often forwarded to people who cannot open it: the warehouse, the finance team and the integrator all need to read it, and none of them run Microsoft Project.",
			"Structure, dependencies and calendar are real; the content is invented. Three calendar exceptions - two bank holidays and a site closure - and the dates shift with the calendar every week, so progress is always measured against today rather than against the day the file was made.",
		},
		anchor: anchoredOn(2026, time.May, 13),
	},
}

func (e Example) Description() string {
	return fmt.Sprintf("%s - %s. A sample MS Project plan you can open in the browser.", e.Label, e.Note)
}

func All() []Example {
	return catalogue
}

func ByName(name string) (Example, bool) {
	for _, e := range catalogue {
		if e.Name == name {
			return e, true
		}
	}
	return Example{}, false
}

func weekStart(t time.Time) time.Time {
	t = t.UTC()
	back := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-back, 0, 0, 0, 0, time.UTC)
}

func NextWeek(now time.Time) time.Time {
	return weekStart(now).AddDate(0, 0, 7)
}

var (
	mu     sync.Mutex
	cached = map[string][]byte{}
)

func (e Example) Contract(now time.Time) ([]byte, error) {
	days := int(weekStart(now).Sub(weekStart(e.anchor)).Hours() / 24)
	key := fmt.Sprintf("%s@%d", e.Name, days)

	mu.Lock()
	defer mu.Unlock()

	if ready, ok := cached[key]; ok {
		return ready, nil
	}

	raw, err := files.ReadFile(e.Name + ".json")
	if err != nil {
		return nil, err
	}

	c, err := contract.Decode(raw)
	if err != nil {
		return nil, err
	}

	shift(c, days)

	moved, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	cached[key] = moved

	return moved, nil
}

func shift(c *contract.Contract, days int) {
	move := func(v *string) {
		if v == nil {
			return
		}
		t, err := time.Parse(contract.DateLayout, *v)
		if err != nil {
			return
		}
		*v = t.AddDate(0, 0, days).Format(contract.DateLayout)
	}

	move(c.Project.Start)
	move(c.Project.Finish)

	for i := range c.Calendar.Exceptions {
		move(c.Calendar.Exceptions[i].From)
		move(c.Calendar.Exceptions[i].To)
	}

	for i := range c.Tasks {
		move(c.Tasks[i].Start)
		move(c.Tasks[i].Finish)

		if b := c.Tasks[i].Baseline; b != nil {
			move(b.Start)
			move(b.Finish)
		}
	}
}

func (e Example) Format() Format {
	switch {
	case strings.HasSuffix(strings.ToLower(e.FileName), ".xml"):
		return Format{Path: "/open-microsoft-project-xml", Label: "a Project .xml export"}
	case strings.HasSuffix(strings.ToLower(e.FileName), ".mpx"):
		return Format{Path: "/open-mpp-file-without-ms-project", Label: "an .mpx export"}
	default:
		return Format{Path: "/open-mpp-file-without-ms-project", Label: "an .mpp file"}
	}
}

var (
	statsMu     sync.Mutex
	statsCached = map[string]Stats{}
)

func (e Example) Stats() Stats {
	statsMu.Lock()
	defer statsMu.Unlock()

	if ready, ok := statsCached[e.Name]; ok {
		return ready
	}

	raw, err := files.ReadFile(e.Name + ".json")
	if err != nil {
		return Stats{}
	}

	c, err := contract.Decode(raw)
	if err != nil {
		return Stats{}
	}

	stats := Stats{
		Resources:  len(c.Resources),
		Tasks:      len(c.Tasks),
		Relations:  len(c.Relations),
		Exceptions: len(c.Calendar.Exceptions),
	}

	for _, task := range c.Tasks {
		if task.IsSummary {
			stats.Summaries++
		}
		if task.IsMilestone {
			stats.Milestones++
		}
		if task.IsCritical {
			stats.Critical++
		}
		if task.OutlineLevel > stats.Depth {
			stats.Depth = task.OutlineLevel
		}
		stats.Assignments += len(task.Assignments)
	}

	statsCached[e.Name] = stats

	return stats
}
