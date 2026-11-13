package viewer

import (
	"encoding/json"
	"math"
	"net/http"
	"server/internal/diff"
	"server/internal/examples"
	"server/internal/htmlutil"
	"server/internal/user"
	"sync"
	"time"
)

const (
	comparisonSlug  = "/schedule-comparison"
	comparisonMoved = 5
	contractDate    = "2006-01-02T15:04:05"
)

type MovedTask struct {
	Name string
	Days int64
}

type ComparisonExample struct {
	PreviousName string
	CurrentName  string
	FinishOld    string
	FinishNew    string
	FinishDays   int64
	Added        []string
	Removed      []string
	Edited       int
	Links        int
	MovedCount   int
	Moved        []MovedTask
	Entered      []string
	Left         []string
	Overlap      int
	Pair         string
}

var comparison struct {
	once    sync.Once
	example ComparisonExample
	err     error
}

func comparisonExample() (ComparisonExample, error) {
	comparison.once.Do(func() {
		comparison.example, comparison.err = buildComparisonExample()
	})
	return comparison.example, comparison.err
}

func buildComparisonExample() (ComparisonExample, error) {
	p, err := examples.ComparisonPair()
	if err != nil {
		return ComparisonExample{}, err
	}

	r := diff.Compare(p.Previous, p.Current)

	pair, err := json.Marshal(struct {
		Previous     json.RawMessage `json:"previous"`
		Current      json.RawMessage `json:"current"`
		PreviousName string          `json:"previous_name"`
		CurrentName  string          `json:"current_name"`
	}{p.PreviousRaw, p.CurrentRaw, p.PreviousName, p.CurrentName})
	if err != nil {
		return ComparisonExample{}, err
	}

	e := ComparisonExample{
		PreviousName: p.PreviousName,
		CurrentName:  p.CurrentName,
		Added:        names(r.Added),
		Removed:      names(r.Removed),
		Edited:       len(r.Edited),
		Links:        len(r.Links),
		MovedCount:   len(r.Moved),
		Entered:      names(r.Entered),
		Left:         names(r.Left),
		Overlap:      int(math.Round(diff.Overlap(p.Previous, p.Current) * 100)),
		Pair:         string(pair),
	}

	if r.Finish != nil {
		e.FinishOld = day(r.Finish.Old)
		e.FinishNew = day(r.Finish.New)
		e.FinishDays = days(r.Finish.Minutes)
	}

	for _, m := range r.Moved {
		if len(e.Moved) == comparisonMoved {
			break
		}
		shift := m.Finish
		if shift == nil {
			shift = m.Start
		}
		e.Moved = append(e.Moved, MovedTask{Name: m.Task.Name, Days: days(shift.Minutes)})
	}

	return e, nil
}

func names(refs []diff.Ref) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Name)
	}
	return out
}

func day(raw *string) string {
	if raw == nil {
		return ""
	}
	t, err := time.Parse(contractDate, *raw)
	if err != nil {
		return *raw
	}
	return t.Format("2 Jan 2006")
}

func days(minutes int64) int64 {
	return int64(math.Round(float64(minutes) / (24 * 60)))
}

func (h *Handler) ComparisonPage(w http.ResponseWriter, r *http.Request) {
	example, err := comparisonExample()
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.landingPage(w, r, comparisonSlug, h.templates.Comparison, user.NewPage(r, example))
}
