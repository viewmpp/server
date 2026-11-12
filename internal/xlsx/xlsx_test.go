package xlsx

import (
	"bytes"
	"server/internal/assert"
	"server/internal/contract"
	"server/internal/fixtures"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
)

func TestWriteFixtures(t *testing.T) {
	for name, raw := range fixtures.All() {
		t.Run(name, func(t *testing.T) {
			c, err := contract.Decode(raw)
			assert.NilError(t, err)

			var buf bytes.Buffer
			assert.NilError(t, Write(&buf, c))

			f, err := excelize.OpenReader(&buf)
			assert.NilError(t, err)
			defer f.Close()

			rows, err := f.GetRows(sheet)
			assert.NilError(t, err)

			if got, want := len(rows), len(c.Tasks)+1; got != want {
				t.Fatalf("got %d rows, want %d (header + tasks)", got, want)
			}

			if rows[0][0] != "ID" {
				t.Errorf("header starts with %q, want ID", rows[0][0])
			}
		})
	}
}

func TestWriteKeepsCyrillic(t *testing.T) {
	c, err := contract.Decode(fixtures.All()["cyrillic.json"])
	assert.NilError(t, err)

	var buf bytes.Buffer
	if err = Write(&buf, c); err != nil {
		t.Fatalf("write: %v", err)
	}

	f, err := excelize.OpenReader(&buf)
	assert.NilError(t, err)
	defer f.Close()

	rows, err := f.GetRows(sheet)
	assert.NilError(t, err)

	var joined strings.Builder
	for _, row := range rows {
		joined.WriteString(strings.Join(row, " "))
	}

	if !strings.Contains(joined.String(), "Фаза") {
		t.Error("cyrillic task names did not survive the round trip")
	}
}

func TestWriteKeepsDatesAsDates(t *testing.T) {
	c, err := contract.Decode(fixtures.All()["mpp14baseline.json"])
	assert.NilError(t, err)

	var buf bytes.Buffer
	if err = Write(&buf, c); err != nil {
		t.Fatalf("write: %v", err)
	}

	f, err := excelize.OpenReader(&buf)
	assert.NilError(t, err)
	defer f.Close()

	got, err := f.GetCellValue(sheet, cell(startColumn, 2))
	assert.NilError(t, err)

	if got == "" || strings.Contains(got, "T") {
		t.Errorf("start cell is %q; expected a formatted date, not the raw ISO string", got)
	}
}

func TestIndentIsBounded(t *testing.T) {
	name := "task"
	got := indent(contract.Task{Name: &name, OutlineLevel: contract.MaxOutlineLevel})
	want := strings.Repeat("    ", maxVisibleOutlineLevel-1) + name
	if got != want {
		t.Fatalf("indent length is %d, want %d", len(got), len(want))
	}
}

func TestTheLongestPredecessorCellFitsInOneExcelCell(t *testing.T) {
	lag := &contract.Amount{Value: -contract.MaxNumber, Units: "ELAPSED_MINUTES"}

	relations := make([]contract.Relation, contract.MaxPredecessorsPerTask)
	for i := range relations {
		relations[i] = contract.Relation{
			ID:            int64(i + 1),
			PredecessorID: contract.MaxID,
			SuccessorID:   1,
			Type:          "START_FINISH",
			Lag:           lag,
		}
	}

	longest := predecessorList(relations, map[int64]int{contract.MaxID: contract.MaxTasks})

	if got := len(utf16.Encode([]rune(longest))); got > excelize.TotalCellChars {
		t.Fatalf("%d predecessors make a %d unit cell; excelize truncates anything over %d without an error", contract.MaxPredecessorsPerTask, got, excelize.TotalCellChars)
	}
}

func sheetRows(t *testing.T, c *contract.Contract) [][]string {
	t.Helper()

	var buf bytes.Buffer
	assert.NilError(t, Write(&buf, c))

	f, err := excelize.OpenReader(&buf)
	assert.NilError(t, err)
	defer f.Close()

	rows, err := f.GetRows(sheet)
	assert.NilError(t, err)

	return rows
}

func column(t *testing.T, header string) int {
	t.Helper()

	for i, h := range headers {
		if h == header {
			return i
		}
	}

	t.Fatalf("no %s column", header)
	return -1
}

func TestPredecessorsPointAtTheIDColumnRatherThanTheUniqueID(t *testing.T) {
	c := &contract.Contract{
		Tasks: []contract.Task{{ID: 0}, {ID: 1}, {ID: 7}, {ID: 2}},
		Relations: []contract.Relation{
			{ID: 1, PredecessorID: 7, SuccessorID: 2, Type: "FINISH_START"},
			{ID: 2, PredecessorID: 1, SuccessorID: 2, Type: "START_START"},
		},
	}

	rows := sheetRows(t, c)

	if got := rows[4][column(t, "Predecessors")]; got != "2, 1 SS" {
		t.Errorf("Predecessors = %q, want %q: task 7 sits in the row with ID 2, and an MS Project reader looks for 2", got, "2, 1 SS")
	}
}

func TestAPredecessorMissingFromThePlanIsLeftOut(t *testing.T) {
	c := &contract.Contract{
		Tasks: []contract.Task{{ID: 0}, {ID: 1}, {ID: 2}},
		Relations: []contract.Relation{
			{ID: 1, PredecessorID: 99, SuccessorID: 2, Type: "FINISH_START"},
			{ID: 2, PredecessorID: 1, SuccessorID: 2, Type: "FINISH_START"},
		},
	}

	rows := sheetRows(t, c)

	if got := rows[3][column(t, "Predecessors")]; got != "1" {
		t.Errorf("Predecessors = %q, want %q: no row has the number 99", got, "1")
	}
}

func TestTheIDColumnNumbersRowsAsMSProjectDoes(t *testing.T) {
	tests := map[string]int{
		"mspdi.json": 0,
		"mpp8.json":  1,
	}

	for name, first := range tests {
		t.Run(name, func(t *testing.T) {
			c, err := contract.Decode(fixtures.All()[name])
			assert.NilError(t, err)

			rows := sheetRows(t, c)

			for i, row := range rows[1:] {
				if want := strconv.Itoa(first + i); row[0] != want {
					t.Fatalf("row %d has ID %q, want %s", i+1, row[0], want)
				}
			}
		})
	}
}

func TestTheWBSColumnCarriesTheOutlineNumber(t *testing.T) {
	node, outline, name := "HOSPEXP.1", "1.2.3", "Excavation"

	c := &contract.Contract{
		Tasks: []contract.Task{
			{ID: 1, Name: &name, WBS: &node, OutlineNumber: &outline},
		},
	}

	var buf bytes.Buffer
	assert.NilError(t, Write(&buf, c))

	f, err := excelize.OpenReader(&buf)
	assert.NilError(t, err)
	defer f.Close()

	rows, err := f.GetRows(sheet)
	assert.NilError(t, err)

	if got := rows[1][1]; got != outline {
		t.Errorf("WBS column holds %q, want the outline number %q: /mpp-to-excel promises a value such as 1.2.3, and the breakdown node code repeats for every task inside it", got, outline)
	}
}
