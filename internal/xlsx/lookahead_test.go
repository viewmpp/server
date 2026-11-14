package xlsx

import (
	"bytes"
	"server/internal/contract"
	"server/internal/lookahead"
	"strconv"
	"testing"

	"github.com/xuri/excelize/v2"
)

func lookaheadPlan() *contract.Contract {
	s := func(v string) *string { return &v }
	root, phase := int64(0), int64(1)
	return &contract.Contract{
		Resources: []contract.Resource{{ID: 1, Name: "Electrical Co"}, {ID: 2, Name: "Drylining Ltd"}},
		Tasks: []contract.Task{
			{ID: 0, Name: s("Refurbishment"), IsSummary: true, Start: s("2026-03-02T08:00:00"), Finish: s("2026-06-12T17:00:00")},
			{ID: 1, ParentID: &root, Name: s("First fix"), OutlineLevel: 1, IsSummary: true, Start: s("2026-03-02T08:00:00"), Finish: s("2026-05-01T17:00:00")},
			{ID: 2, ParentID: &phase, Name: s("Survey"), OutlineLevel: 2, Start: s("2026-03-02T08:00:00"), Finish: s("2026-03-06T17:00:00")},
			{ID: 3, ParentID: &phase, Name: s("Electrical first fix"), OutlineLevel: 2, Start: s("2026-04-07T08:00:00"), Finish: s("2026-04-16T17:00:00"),
				Assignments: []contract.Assignment{{ResourceID: 1}}},
			{ID: 4, ParentID: &phase, Name: s("Partitions"), OutlineLevel: 2, Start: s("2026-04-14T08:00:00"), Finish: s("2026-04-22T17:00:00"),
				Assignments: []contract.Assignment{{ResourceID: 2}, {ResourceID: 1}}},
			{ID: 5, ParentID: &phase, Name: s("Site meeting"), OutlineLevel: 2, Start: s("2026-04-08T08:00:00"), Finish: s("2026-04-08T17:00:00")},
		},
	}
}

func openLookahead(t *testing.T, plan *contract.Contract) *excelize.File {
	t.Helper()

	w, err := lookahead.Parse("2026-04-06", "3")
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err = WriteLookahead(&buf, plan, w); err != nil {
		t.Fatal(err)
	}

	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func lookaheadRows(t *testing.T, f *excelize.File, sheet string) [][]string {
	t.Helper()
	out, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("sheet %s: %v", sheet, err)
	}
	return out
}

func TestTheLookaheadKeepsTheRowNumbersOfTheWholePlan(t *testing.T) {
	got := lookaheadRows(t, openLookahead(t, lookaheadPlan()), lookaheadSheet)

	var ids []string
	for _, r := range got[1:] {
		ids = append(ids, r[0])
	}

	want := []string{"0", "1", "3", "4", "5"}
	if len(ids) != len(want) {
		t.Fatalf("ids %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("row %d has ID %s, want %s: the ID must match the plan in MS Project, not the position in the lookahead", i, ids[i], want[i])
		}
	}
}

func TestTheLookaheadListsEachResourcesWork(t *testing.T) {
	got := lookaheadRows(t, openLookahead(t, lookaheadPlan()), resourceSheet)

	var lines []string
	for _, r := range got[1:] {
		lines = append(lines, r[0]+" | "+r[2]+" | "+r[3])
	}

	want := []string{
		"Electrical Co | Electrical first fix | First fix",
		"Electrical Co | Partitions | First fix",
		"Drylining Ltd | Partitions | First fix",
		"No resource | Site meeting | First fix",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestAPlanWithoutResourcesGetsNoResourceSheet(t *testing.T) {
	plan := lookaheadPlan()
	plan.Resources = nil
	for i := range plan.Tasks {
		plan.Tasks[i].Assignments = nil
	}

	if idx, _ := openLookahead(t, plan).GetSheetIndex(resourceSheet); idx != -1 {
		t.Error("a By resource sheet with nothing but No resource in it")
	}
}

func TestASummaryRowKeepsItsDatesAsDates(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, lookaheadPlan()); err != nil {
		t.Fatal(err)
	}
	full, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}

	for name, f := range map[string]*excelize.File{"full export": full, "lookahead": openLookahead(t, lookaheadPlan())} {
		sheet := full.GetSheetName(0)
		if name == "lookahead" {
			sheet = lookaheadSheet
		}
		for _, axis := range []string{"D2", "E2"} {
			value, err := f.GetCellValue(sheet, axis)
			if err != nil {
				t.Fatal(err)
			}
			if _, isNumber := strconv.ParseFloat(value, 64); isNumber == nil {
				t.Errorf("%s %s shows %q: the bold of a summary row wiped the date format", name, axis, value)
			}
		}
	}
}
