package diff

import (
	"cmp"
	"math"
	"server/internal/contract"
	"slices"
	"time"
)

type Ref struct {
	ID      int64  `json:"id"`
	Row     int    `json:"row"`
	Name    string `json:"name"`
	Outline string `json:"outline,omitempty"`
}

type Change[T any] struct {
	Old T `json:"old"`
	New T `json:"new"`
}

type Shift struct {
	Old     *string `json:"old"`
	New     *string `json:"new"`
	Minutes int64   `json:"minutes"`
}

type Edit struct {
	Task      Ref                       `json:"task"`
	Name      *Change[string]           `json:"name,omitempty"`
	Parent    *Change[*Ref]             `json:"parent,omitempty"`
	Duration  *Change[*contract.Amount] `json:"duration,omitempty"`
	Complete  *Change[float64]          `json:"percent_complete,omitempty"`
	Milestone *Change[bool]             `json:"milestone,omitempty"`
}

type Move struct {
	Task   Ref    `json:"task"`
	Start  *Shift `json:"start,omitempty"`
	Finish *Shift `json:"finish,omitempty"`
}

type Link struct {
	Type string           `json:"type"`
	Lag  *contract.Amount `json:"lag"`
}

type LinkChange struct {
	Predecessor Ref   `json:"predecessor"`
	Successor   Ref   `json:"successor"`
	Old         *Link `json:"old"`
	New         *Link `json:"new"`
}

type Report struct {
	Start   *Shift       `json:"start"`
	Finish  *Shift       `json:"finish"`
	Added   []Ref        `json:"added"`
	Removed []Ref        `json:"removed"`
	Edited  []Edit       `json:"edited"`
	Links   []LinkChange `json:"links"`
	Moved   []Move       `json:"moved"`
	Entered []Ref        `json:"entered_critical"`
	Left    []Ref        `json:"left_critical"`
	Rollups []Move       `json:"rollups"`
}

type version struct {
	contract *contract.Contract
	tasks    map[int64]*contract.Task
	rows     map[int64]int
	project  *int64
}

func newVersion(c *contract.Contract) *version {
	v := &version{
		contract: c,
		tasks:    make(map[int64]*contract.Task, len(c.Tasks)),
		rows:     c.RowIDs(),
	}

	var roots []*contract.Task
	for i := range c.Tasks {
		t := &c.Tasks[i]
		if _, seen := v.tasks[t.ID]; !seen {
			v.tasks[t.ID] = t
		}
		if t.ParentID == nil {
			roots = append(roots, t)
		}
	}
	if len(roots) == 1 && roots[0].IsSummary {
		v.project = &roots[0].ID
	}

	return v
}

func (v *version) isProject(id int64) bool {
	return v.project != nil && *v.project == id
}

func (v *version) ref(t *contract.Task) Ref {
	return Ref{ID: t.ID, Row: v.rows[t.ID], Name: deref(t.Name), Outline: deref(t.OutlineNumber)}
}

func (v *version) parentRef(t *contract.Task) *Ref {
	if t.ParentID == nil || v.isProject(*t.ParentID) {
		return nil
	}
	parent, ok := v.tasks[*t.ParentID]
	if !ok {
		return nil
	}
	r := v.ref(parent)
	return &r
}

func (v *version) projectDates() (start, finish *string) {
	start, finish = v.contract.Project.Start, v.contract.Project.Finish
	if v.project != nil {
		row := v.tasks[*v.project]
		if start == nil {
			start = row.Start
		}
		if finish == nil {
			finish = row.Finish
		}
	}
	return start, finish
}

func Compare(previous, current *contract.Contract) Report {
	old, cur := newVersion(previous), newVersion(current)

	r := Report{
		Added:   []Ref{},
		Removed: []Ref{},
		Edited:  []Edit{},
		Links:   []LinkChange{},
		Moved:   []Move{},
		Entered: []Ref{},
		Left:    []Ref{},
		Rollups: []Move{},
	}

	oldStart, oldFinish := old.projectDates()
	curStart, curFinish := cur.projectDates()
	r.Start = shift(oldStart, curStart)
	r.Finish = shift(oldFinish, curFinish)

	for i := range current.Tasks {
		t := &current.Tasks[i]
		if cur.tasks[t.ID] != t || cur.isProject(t.ID) {
			continue
		}

		before, existed := old.tasks[t.ID]
		if !existed {
			r.Added = append(r.Added, cur.ref(t))
			continue
		}
		if old.isProject(t.ID) {
			continue
		}

		compareTask(&r, old, cur, before, t)
	}

	for i := range previous.Tasks {
		t := &previous.Tasks[i]
		if old.tasks[t.ID] != t || old.isProject(t.ID) {
			continue
		}
		if _, kept := cur.tasks[t.ID]; !kept {
			r.Removed = append(r.Removed, old.ref(t))
		}
	}

	r.Links = compareLinks(old, cur)

	byMagnitude := func(a, b Move) int {
		return cmp.Or(
			cmp.Compare(magnitude(b.Finish), magnitude(a.Finish)),
			cmp.Compare(magnitude(b.Start), magnitude(a.Start)),
		)
	}
	slices.SortStableFunc(r.Moved, byMagnitude)
	slices.SortStableFunc(r.Rollups, byMagnitude)

	return r
}

func Overlap(previous, current *contract.Contract) float64 {
	names := make(map[int64]string, len(previous.Tasks))
	for _, t := range previous.Tasks {
		if _, seen := names[t.ID]; !seen {
			names[t.ID] = deref(t.Name)
		}
	}

	shared := make(map[int64]bool, len(current.Tasks))
	for _, t := range current.Tasks {
		if name, ok := names[t.ID]; ok && name == deref(t.Name) {
			shared[t.ID] = true
		}
	}

	larger := max(len(names), len(current.Tasks))
	if larger == 0 {
		return 1
	}
	return float64(len(shared)) / float64(larger)
}

func compareTask(r *Report, old, cur *version, before, after *contract.Task) {
	edit := Edit{Task: cur.ref(after)}
	edited := false

	if deref(before.Name) != deref(after.Name) {
		edit.Name = &Change[string]{Old: deref(before.Name), New: deref(after.Name)}
		edited = true
	}

	oldParent, newParent := old.parentRef(before), cur.parentRef(after)
	if parentID(oldParent) != parentID(newParent) {
		edit.Parent = &Change[*Ref]{Old: oldParent, New: newParent}
		edited = true
	}

	summary := before.IsSummary || after.IsSummary

	if !summary {
		if !sameAmount(before.Duration, after.Duration) {
			edit.Duration = &Change[*contract.Amount]{Old: before.Duration, New: after.Duration}
			edited = true
		}
		if !near(before.PercentComplete, after.PercentComplete) {
			edit.Complete = &Change[float64]{Old: before.PercentComplete, New: after.PercentComplete}
			edited = true
		}
		if before.IsMilestone != after.IsMilestone {
			edit.Milestone = &Change[bool]{Old: before.IsMilestone, New: after.IsMilestone}
			edited = true
		}
	}

	if edited {
		r.Edited = append(r.Edited, edit)
	}

	move := Move{Task: cur.ref(after), Start: shift(before.Start, after.Start), Finish: shift(before.Finish, after.Finish)}
	if move.Start != nil || move.Finish != nil {
		if summary {
			r.Rollups = append(r.Rollups, move)
		} else {
			r.Moved = append(r.Moved, move)
		}
	}

	if !summary && before.IsCritical != after.IsCritical {
		if after.IsCritical {
			r.Entered = append(r.Entered, cur.ref(after))
		} else {
			r.Left = append(r.Left, cur.ref(after))
		}
	}
}

type pair struct {
	predecessor int64
	successor   int64
}

func links(c *contract.Contract) (map[pair]*contract.Relation, []pair) {
	byPair := make(map[pair]*contract.Relation, len(c.Relations))
	order := make([]pair, 0, len(c.Relations))
	for i := range c.Relations {
		rel := &c.Relations[i]
		key := pair{rel.PredecessorID, rel.SuccessorID}
		if _, seen := byPair[key]; seen {
			continue
		}
		byPair[key] = rel
		order = append(order, key)
	}
	return byPair, order
}

func compareLinks(old, cur *version) []LinkChange {
	before, beforeOrder := links(old.contract)
	after, afterOrder := links(cur.contract)

	inBoth := func(id int64) bool {
		_, a := old.tasks[id]
		_, b := cur.tasks[id]
		return a && b
	}

	var out []LinkChange
	add := func(key pair, was, is *contract.Relation) {
		if !inBoth(key.predecessor) || !inBoth(key.successor) {
			return
		}
		out = append(out, LinkChange{
			Predecessor: cur.ref(cur.tasks[key.predecessor]),
			Successor:   cur.ref(cur.tasks[key.successor]),
			Old:         link(was),
			New:         link(is),
		})
	}

	for _, key := range afterOrder {
		is := after[key]
		was, existed := before[key]
		switch {
		case !existed:
			add(key, nil, is)
		case was.Type != is.Type || !sameAmount(was.Lag, is.Lag):
			add(key, was, is)
		}
	}
	for _, key := range beforeOrder {
		if _, kept := after[key]; !kept {
			add(key, before[key], nil)
		}
	}

	if out == nil {
		return []LinkChange{}
	}
	return out
}

func link(rel *contract.Relation) *Link {
	if rel == nil {
		return nil
	}
	return &Link{Type: rel.Type, Lag: rel.Lag}
}

func shift(old, cur *string) *Shift {
	if deref(old) == deref(cur) && (old == nil) == (cur == nil) {
		return nil
	}

	s := &Shift{Old: old, New: cur}
	if old != nil && cur != nil {
		a, errA := time.Parse(contract.DateLayout, *old)
		b, errB := time.Parse(contract.DateLayout, *cur)
		if errA == nil && errB == nil {
			s.Minutes = int64(b.Sub(a) / time.Minute)
		}
	}
	return s
}

func magnitude(s *Shift) int64 {
	if s == nil {
		return 0
	}
	if s.Minutes < 0 {
		return -s.Minutes
	}
	return s.Minutes
}

func parentID(r *Ref) int64 {
	if r == nil {
		return -1
	}
	return r.ID
}

var workingMinutes = map[string]float64{
	"MINUTES": 1,
	"HOURS":   60,
	"DAYS":    480,
	"WEEKS":   2400,
	"MONTHS":  9600,
	"YEARS":   115200,
}

var elapsedMinutes = map[string]float64{
	"ELAPSED_MINUTES": 1,
	"ELAPSED_HOURS":   60,
	"ELAPSED_DAYS":    1440,
	"ELAPSED_WEEKS":   10080,
	"ELAPSED_MONTHS":  43200,
	"ELAPSED_YEARS":   525600,
}

func sameAmount(a, b *contract.Amount) bool {
	if isZero(a) && isZero(b) {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Units == b.Units {
		return near(a.Value, b.Value)
	}
	for _, scale := range []map[string]float64{workingMinutes, elapsedMinutes} {
		x, okA := scale[a.Units]
		y, okB := scale[b.Units]
		if okA && okB {
			return near(a.Value*x, b.Value*y)
		}
	}
	return false
}

func isZero(a *contract.Amount) bool {
	return a == nil || a.Value == 0
}

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*max(1, math.Abs(a), math.Abs(b))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
