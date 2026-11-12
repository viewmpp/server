package diff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"server/internal/assert"
	"server/internal/contract"
	"strings"
	"testing"
)

func load(t *testing.T, name string) *contract.Contract {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", name))
	assert.NilError(t, err)

	c, err := contract.Decode(raw)
	assert.NilError(t, err)

	return c
}

func editOf(r Report, id int64) *Edit {
	for i := range r.Edited {
		if r.Edited[i].Task.ID == id {
			return &r.Edited[i]
		}
	}
	return nil
}

func moveOf(moves []Move, id int64) *Move {
	for i := range moves {
		if moves[i].Task.ID == id {
			return &moves[i]
		}
	}
	return nil
}

func linkOf(r Report, predecessor, successor int64) *LinkChange {
	for i := range r.Links {
		if r.Links[i].Predecessor.ID == predecessor && r.Links[i].Successor.ID == successor {
			return &r.Links[i]
		}
	}
	return nil
}

func TestAnUpdateSavedByMSProjectReadsAsThePlannerMadeIt(t *testing.T) {
	r := Compare(load(t, "hospital-v1.json"), load(t, "hospital-v2.json"))

	if r.Finish == nil || r.Finish.Minutes != 24*60 {
		t.Fatalf("project finish shift = %+v, want one day later", r.Finish)
	}

	if len(r.Added) != 1 || r.Added[0].ID != 99 || r.Added[0].Name != "Dewatering" || r.Added[0].Row != 15 {
		t.Errorf("added = %+v, want Dewatering, unique ID 99, in row 15", r.Added)
	}
	if len(r.Removed) != 1 || r.Removed[0].ID != 56 || r.Removed[0].Name != "Install AC" {
		t.Errorf("removed = %+v, want Install AC", r.Removed)
	}

	if len(r.Edited) != 3 {
		t.Errorf("got %d edits, want the 3 the saved file holds: %+v", len(r.Edited), r.Edited)
	}
	if e := editOf(r, 29); e == nil || e.Duration == nil || e.Duration.Old.Value != 7 || e.Duration.New.Value != 10 {
		t.Errorf("Concrete Second Floor edit = %+v, want duration 7 -> 10 days", e)
	}
	if e := editOf(r, 19); e != nil {
		t.Errorf("Form and Pour Slab edit = %+v; MS Project did not keep the 50%% set through automation, so the file has none", e)
	}
	if e := editOf(r, 27); e == nil || e.Complete == nil || e.Complete.New != 100 {
		t.Errorf("Concrete Basement Slab edit = %+v, want 100%% complete", e)
	}
	if e := editOf(r, 75); e == nil || e.Name == nil || e.Name.New != "Drywall in Patient Rooms and Corridors" {
		t.Errorf("drywall edit = %+v, want the new name", e)
	}
	if e := editOf(r, 12); e != nil {
		t.Errorf("the Foundation summary is listed as edited (%+v); its percent only rolled up from the inserted task", e)
	}

	if len(r.Links) != 3 {
		t.Errorf("got %d link changes, want 3: %+v", len(r.Links), r.Links)
	}
	if l := linkOf(r, 75, 78); l == nil || l.Old.Lag.Value != 56 || l.New.Lag.Value != 80 {
		t.Errorf("painting lag change = %+v, want SS+56h -> SS+80h", l)
	}
	if l := linkOf(r, 63, 62); l == nil || l.Old.Type != "FINISH_FINISH" || l.New.Type != "FINISH_START" {
		t.Errorf("roof link change = %+v, want FF -> FS", l)
	}
	if l := linkOf(r, 14, 15); l == nil || l.Old == nil || l.New != nil {
		t.Errorf("excavation to water lines = %+v, want removed: Dewatering now sits between them", l)
	}
	if l := linkOf(r, 61, 62); l != nil {
		t.Errorf("brick to roof listed as changed (%+v); MS Project only rewrote a zero lag from hours to days", l)
	}

	if m := moveOf(r.Moved, 29); m == nil || m.Finish == nil || m.Finish.Minutes != 3*24*60 {
		t.Errorf("Concrete Second Floor move = %+v, want its finish three days later", m)
	}
	if moveOf(r.Moved, 22) != nil {
		t.Error("the Structure summary is among moved tasks; summaries belong to rollups")
	}
	if moveOf(r.Rollups, 22) == nil {
		t.Error("the Structure summary moved and is missing from rollups")
	}
	for i := 1; i < len(r.Moved); i++ {
		if magnitude(r.Moved[i-1].Finish) < magnitude(r.Moved[i].Finish) {
			t.Fatalf("moved tasks are not ordered by how far their finish moved, at %d", i)
		}
	}

	entered := false
	for _, ref := range r.Entered {
		entered = entered || ref.ID == 29
	}
	if !entered {
		t.Errorf("Concrete Second Floor became critical and is missing from entered: %+v", r.Entered)
	}
}

func TestTheSamePlanHasNothingToReport(t *testing.T) {
	plan := load(t, "hospital-v1.json")

	r := Compare(plan, plan)

	if r.Start != nil || r.Finish != nil {
		t.Errorf("project dates reported as moved: %+v %+v", r.Start, r.Finish)
	}
	for name, n := range map[string]int{
		"added": len(r.Added), "removed": len(r.Removed), "edited": len(r.Edited), "links": len(r.Links),
		"moved": len(r.Moved), "entered": len(r.Entered), "left": len(r.Left), "rollups": len(r.Rollups),
	} {
		if n != 0 {
			t.Errorf("%s has %d entries comparing a plan with itself", name, n)
		}
	}
}

func TestAnEmptyListIsAnEmptyArrayInJSON(t *testing.T) {
	plan := load(t, "hospital-v1.json")

	raw, err := json.Marshal(Compare(plan, plan))
	assert.NilError(t, err)

	if strings.Contains(string(raw), "null,") && strings.Contains(string(raw), `"added":null`) {
		t.Fatalf("an empty list encodes as null: %s", raw)
	}
	for _, field := range []string{"added", "removed", "edited", "links", "moved", "entered_critical", "left_critical", "rollups"} {
		if !strings.Contains(string(raw), `"`+field+`":[]`) {
			t.Errorf("%s is not an empty array in %s", field, raw)
		}
	}
}

func amount(value float64, units string) *contract.Amount {
	return &contract.Amount{Value: value, Units: units}
}

func TestAmountsAreComparedInOneUnit(t *testing.T) {
	tests := []struct {
		name string
		a, b *contract.Amount
		same bool
	}{
		{"zero in hours and zero in days", amount(0, "HOURS"), amount(0, "DAYS"), true},
		{"no lag and a zero lag", nil, amount(0, "DAYS"), true},
		{"eight working hours and one working day", amount(8, "HOURS"), amount(1, "DAYS"), true},
		{"a day and a day and a half", amount(1, "DAYS"), amount(1.5, "DAYS"), false},
		{"an elapsed day and a working day", amount(1, "ELAPSED_DAYS"), amount(1, "DAYS"), false},
		{"24 elapsed hours and one elapsed day", amount(24, "ELAPSED_HOURS"), amount(1, "ELAPSED_DAYS"), true},
		{"a value read back with float noise", amount(634.325, "DAYS"), amount(634.3250000000001, "DAYS"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, sameAmount(tt.a, tt.b), tt.same)
		})
	}
}

func str(s string) *string { return &s }
func id(n int64) *int64    { return &n }

func plan(tasks []contract.Task, relations ...contract.Relation) *contract.Contract {
	return &contract.Contract{Version: contract.Version, Tasks: tasks, Relations: relations}
}

func TestLinksOfAnAddedOrRemovedTaskAreNotEditsOfTheirOwn(t *testing.T) {
	before := plan(
		[]contract.Task{{ID: 1, Name: str("a")}, {ID: 2, Name: str("b")}, {ID: 3, Name: str("c")}},
		contract.Relation{ID: 1, PredecessorID: 1, SuccessorID: 2, Type: "FINISH_START"},
		contract.Relation{ID: 2, PredecessorID: 2, SuccessorID: 3, Type: "FINISH_START"},
	)
	after := plan(
		[]contract.Task{{ID: 1, Name: str("a")}, {ID: 3, Name: str("c")}, {ID: 4, Name: str("d")}},
		contract.Relation{ID: 1, PredecessorID: 4, SuccessorID: 3, Type: "FINISH_START"},
		contract.Relation{ID: 2, PredecessorID: 1, SuccessorID: 3, Type: "FINISH_START"},
	)

	r := Compare(before, after)

	if len(r.Links) != 1 || r.Links[0].Predecessor.ID != 1 || r.Links[0].Successor.ID != 3 || r.Links[0].Old != nil {
		t.Errorf("links = %+v, want only the new 1 -> 3; the rest come and go with task 2 and task 4", r.Links)
	}
}

func TestATaskMovedToAnotherPhaseIsAnEdit(t *testing.T) {
	before := plan([]contract.Task{
		{ID: 1, Name: str("phase one"), IsSummary: true},
		{ID: 2, Name: str("phase two"), IsSummary: true},
		{ID: 3, Name: str("pour"), ParentID: id(1)},
	})
	after := plan([]contract.Task{
		{ID: 1, Name: str("phase one"), IsSummary: true},
		{ID: 2, Name: str("phase two"), IsSummary: true},
		{ID: 3, Name: str("pour"), ParentID: id(2)},
	})

	r := Compare(before, after)

	e := editOf(r, 3)
	if e == nil || e.Parent == nil || e.Parent.Old.Name != "phase one" || e.Parent.New.Name != "phase two" {
		t.Fatalf("edit = %+v, want pour moved from phase one to phase two", e)
	}
}

func TestATaskTurnedIntoAMilestoneIsAnEdit(t *testing.T) {
	before := plan([]contract.Task{{ID: 1, Name: str("handover"), Duration: amount(1, "DAYS")}})
	after := plan([]contract.Task{{ID: 1, Name: str("handover"), Duration: amount(0, "DAYS"), IsMilestone: true}})

	e := editOf(Compare(before, after), 1)

	if e == nil || e.Milestone == nil || !e.Milestone.New || e.Duration == nil {
		t.Fatalf("edit = %+v, want the milestone flag and the duration", e)
	}
}

func TestAPlanWithoutASummaryRowTakesItsFinishFromTheProject(t *testing.T) {
	before := plan([]contract.Task{{ID: 1, Name: str("a")}, {ID: 2, Name: str("b")}})
	before.Project.Finish = str("2026-03-02T17:00:00")
	after := plan([]contract.Task{{ID: 1, Name: str("a")}, {ID: 2, Name: str("b")}})
	after.Project.Finish = str("2026-03-09T17:00:00")

	r := Compare(before, after)

	if r.Finish == nil || r.Finish.Minutes != 7*24*60 {
		t.Fatalf("finish = %+v, want a week later", r.Finish)
	}
}

func TestTheRowNumberIsTheOneMSProjectShows(t *testing.T) {
	before := plan([]contract.Task{{ID: 0, Name: str("project"), IsSummary: true}, {ID: 5, Name: str("a"), ParentID: id(0)}})
	after := plan([]contract.Task{
		{ID: 0, Name: str("project"), IsSummary: true},
		{ID: 9, Name: str("new"), ParentID: id(0)},
		{ID: 5, Name: str("a renamed"), ParentID: id(0)},
	})

	r := Compare(before, after)

	if e := editOf(r, 5); e == nil || e.Task.Row != 2 {
		t.Fatalf("edit = %+v, want the task found in row 2 of the new version", e)
	}
	if len(r.Added) != 1 || r.Added[0].Row != 1 {
		t.Errorf("added = %+v, want the new task in row 1", r.Added)
	}
}
