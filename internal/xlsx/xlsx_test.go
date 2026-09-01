package xlsx

import (
	"bytes"
	"server/internal/assert"
	"server/internal/contract"
	"server/internal/fixtures"
	"strings"
	"testing"

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

			if rows[0][0] != "WBS" {
				t.Errorf("header starts with %q, want WBS", rows[0][0])
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

	got, err := f.GetCellValue(sheet, "C2")
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
