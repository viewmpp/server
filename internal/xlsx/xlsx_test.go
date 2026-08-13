package xlsx

import (
	"bytes"
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
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			var buf bytes.Buffer
			if err = Write(&buf, c); err != nil {
				t.Fatalf("write: %v", err)
			}

			f, err := excelize.OpenReader(&buf)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			defer f.Close()

			rows, err := f.GetRows(sheet)
			if err != nil {
				t.Fatalf("rows: %v", err)
			}

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
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var buf bytes.Buffer
	if err = Write(&buf, c); err != nil {
		t.Fatalf("write: %v", err)
	}

	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("rows: %v", err)
	}

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
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var buf bytes.Buffer
	if err = Write(&buf, c); err != nil {
		t.Fatalf("write: %v", err)
	}

	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer f.Close()

	got, err := f.GetCellValue(sheet, "C2")
	if err != nil {
		t.Fatalf("cell: %v", err)
	}

	if got == "" || strings.Contains(got, "T") {
		t.Errorf("start cell is %q; expected a formatted date, not the raw ISO string", got)
	}
}
