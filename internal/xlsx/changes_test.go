package xlsx

import (
	"bytes"
	"os"
	"path/filepath"
	"server/internal/assert"
	"server/internal/contract"
	"server/internal/diff"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func hospitalReport(t *testing.T) diff.Report {
	t.Helper()

	load := func(name string) *contract.Contract {
		raw, err := os.ReadFile(filepath.Join("..", "diff", "testdata", name))
		assert.NilError(t, err)
		c, err := contract.Decode(raw)
		assert.NilError(t, err)
		return c
	}

	return diff.Compare(load("hospital-v1.json"), load("hospital-v2.json"))
}

func changesBook(t *testing.T) *excelize.File {
	t.Helper()

	var buf bytes.Buffer
	assert.NilError(t, WriteChanges(&buf, hospitalReport(t), "hospital-v1.mpp", "hospital-v2.mpp"))

	f, err := excelize.OpenReader(&buf)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	return f
}

func findRow(t *testing.T, rows [][]string, cells ...string) []string {
	t.Helper()

	for _, row := range rows {
		match := true
		for _, want := range cells {
			if !slices.Contains(row, want) {
				match = false
				break
			}
		}
		if match {
			return row
		}
	}

	t.Fatalf("no row holds %q", cells)
	return nil
}

func TestTheChangeReportHasASheetPerSection(t *testing.T) {
	f := changesBook(t)

	want := []string{summarySheet, changedSheet, movedSheet, linksSheet, criticalSheet}
	if got := f.GetSheetList(); !slices.Equal(got, want) {
		t.Fatalf("sheets = %v, want %v", got, want)
	}
}

func TestTheSummaryNamesBothFilesAndTheFinishShift(t *testing.T) {
	f := changesBook(t)

	rows, err := f.GetRows(summarySheet)
	assert.NilError(t, err)

	findRow(t, rows, "Previous file", "hospital-v1.mpp")
	findRow(t, rows, "Current file", "hospital-v2.mpp")
	findRow(t, rows, "Project start", "unchanged")

	finish := findRow(t, rows, "Project finish")
	if len(finish) < 4 || finish[3] != "1.00" {
		t.Errorf("finish row = %q, want a shift of 1.00 calendar day", finish)
	}
	if strings.Contains(finish[1], "T") {
		t.Errorf("finish before = %q, want a formatted date rather than the raw string", finish[1])
	}
}

func TestTheChangedSheetListsWhatThePlannerDid(t *testing.T) {
	f := changesBook(t)

	rows, err := f.GetRows(changedSheet)
	assert.NilError(t, err)

	findRow(t, rows, "Dewatering", "Added")
	findRow(t, rows, "Install AC", "Removed")
	findRow(t, rows, "Concrete Second Floor", "Duration", "7 days", "10 days")
	findRow(t, rows, "Drywall in Patient Rooms and Corridors", "Renamed", "Drywall in Patient Rooms")

	progress := findRow(t, rows, "Concrete Basement Slab", "Percent complete")
	if progress[4] != "0%" || progress[5] != "100%" {
		t.Errorf("percent row = %q, want 0%% and 100%% as numbers formatted as percent", progress)
	}
}

func TestTheMovedSheetGivesShiftsAsNumbers(t *testing.T) {
	f := changesBook(t)

	rows, err := f.GetRows(movedSheet)
	assert.NilError(t, err)

	second := findRow(t, rows, "Concrete Second Floor", "Task")
	if second[9] != "3.00" {
		t.Errorf("Concrete Second Floor finish shift = %q, want 3.00", second[9])
	}

	findRow(t, rows, "Structure", "Summary")
}

func TestTheLinksSheetShowsTheRoofLinkTurningToFinishStart(t *testing.T) {
	f := changesBook(t)

	rows, err := f.GetRows(linksSheet)
	assert.NilError(t, err)

	findRow(t, rows, "Install Exterior Doors and Windows", "Insulation and Built-up Roofing", "FF", "FS")
	findRow(t, rows, "Excavation", "Install Underground Water/Sewer Lines", "FS", "none")
}
