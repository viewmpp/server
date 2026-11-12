package xlsx

import (
	"fmt"
	"io"
	"math"
	"server/internal/contract"
	"server/internal/diff"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	summarySheet  = "Summary"
	changedSheet  = "Changed"
	movedSheet    = "Moved"
	linksSheet    = "Links"
	criticalSheet = "Critical path"
)

type book struct {
	f       *excelize.File
	head    int
	date    int
	decimal int
}

func WriteChanges(w io.Writer, r diff.Report, previous, current string) error {
	f := excelize.NewFile()
	defer f.Close()

	b := &book{f: f}
	var err error

	if b.head, err = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F7F8FA"}},
	}); err != nil {
		return err
	}
	if b.date, err = f.NewStyle(&excelize.Style{NumFmt: 22}); err != nil {
		return err
	}
	if b.decimal, err = f.NewStyle(&excelize.Style{NumFmt: 2}); err != nil {
		return err
	}

	steps := []func() error{
		func() error { return b.summary(r, previous, current) },
		func() error { return b.changed(r) },
		func() error { return b.moved(r) },
		func() error { return b.links(r) },
		func() error { return b.critical(r) },
	}
	for _, step := range steps {
		if err = step(); err != nil {
			return err
		}
	}

	f.SetActiveSheet(0)
	if err = f.DeleteSheet("Sheet1"); err != nil {
		return err
	}

	return f.Write(w)
}

func (b *book) sheet(name string, headers []string, widths []float64) error {
	if _, err := b.f.NewSheet(name); err != nil {
		return err
	}
	if err := b.f.SetSheetRow(name, "A1", &headers); err != nil {
		return err
	}
	if err := b.f.SetCellStyle(name, "A1", cell(len(headers), 1), b.head); err != nil {
		return err
	}
	for i, width := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := b.f.SetColWidth(name, col, col, width); err != nil {
			return err
		}
	}
	return b.f.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

func (b *book) filter(name string, columns, rows int) error {
	if rows == 0 {
		return nil
	}
	return b.f.AutoFilter(name, fmt.Sprintf("A1:%s", cell(columns, rows+1)), nil)
}

func (b *book) row(name string, row int, values []any) error {
	return b.f.SetSheetRow(name, cell(1, row), &values)
}

func (b *book) summary(r diff.Report, previous, current string) error {
	if err := b.sheet(summarySheet, []string{"", "Before", "Now", "Shift, calendar days"}, []float64{26, 22, 22, 20}); err != nil {
		return err
	}

	rows := [][]any{
		{"Previous file", previous},
		{"Current file", current},
	}
	for _, line := range []struct {
		label string
		shift *diff.Shift
	}{{"Project start", r.Start}, {"Project finish", r.Finish}} {
		if line.shift == nil {
			rows = append(rows, []any{line.label, "unchanged"})
			continue
		}
		rows = append(rows, []any{line.label, nil, nil, days(line.shift)})
	}

	slipped := 0
	for _, m := range r.Moved {
		if m.Finish != nil && m.Finish.Minutes > 0 {
			slipped++
		}
	}

	rows = append(rows,
		[]any{},
		[]any{"Tasks added", len(r.Added)},
		[]any{"Tasks removed", len(r.Removed)},
		[]any{"Tasks edited", len(r.Edited)},
		[]any{"Links changed", len(r.Links)},
		[]any{"Tasks moved", len(r.Moved)},
		[]any{"Tasks finishing later", slipped},
		[]any{"Became critical", len(r.Entered)},
		[]any{"No longer critical", len(r.Left)},
		[]any{},
		[]any{"Shifts are calendar days. The report lists what differs between the two files, not why it changed."},
	)

	for i, values := range rows {
		if err := b.row(summarySheet, i+2, values); err != nil {
			return err
		}
	}

	for i, shift := range []*diff.Shift{r.Start, r.Finish} {
		if shift == nil {
			continue
		}
		row := i + 4
		if err := b.dateCell(summarySheet, cell(2, row), shift.Old); err != nil {
			return err
		}
		if err := b.dateCell(summarySheet, cell(3, row), shift.New); err != nil {
			return err
		}
		if err := b.f.SetCellStyle(summarySheet, cell(4, row), cell(4, row), b.decimal); err != nil {
			return err
		}
	}

	return nil
}

func (b *book) changed(r diff.Report) error {
	headers := []string{"Row", "Outline", "Task", "Change", "Before", "Now"}
	if err := b.sheet(changedSheet, headers, []float64{7, 10, 46, 18, 28, 28}); err != nil {
		return err
	}

	var rows [][]any
	for _, ref := range r.Added {
		rows = append(rows, []any{ref.Row, ref.Outline, ref.Name, "Added", "", ""})
	}
	for _, ref := range r.Removed {
		rows = append(rows, []any{ref.Row, ref.Outline, ref.Name, "Removed", "", ""})
	}
	for _, e := range r.Edited {
		line := func(change string, before, now any) []any {
			return []any{e.Task.Row, e.Task.Outline, e.Task.Name, change, before, now}
		}
		if e.Name != nil {
			rows = append(rows, line("Renamed", e.Name.Old, e.Name.New))
		}
		if e.Parent != nil {
			rows = append(rows, line("Moved to another phase", phase(e.Parent.Old), phase(e.Parent.New)))
		}
		if e.Duration != nil {
			rows = append(rows, line("Duration", amountText(e.Duration.Old), amountText(e.Duration.New)))
		}
		if e.Complete != nil {
			rows = append(rows, line("Percent complete", e.Complete.Old/100, e.Complete.New/100))
		}
		if e.Milestone != nil {
			rows = append(rows, line("Milestone", flagText(e.Milestone.Old), flagText(e.Milestone.New)))
		}
	}

	percent, err := b.f.NewStyle(&excelize.Style{NumFmt: 9})
	if err != nil {
		return err
	}

	for i, values := range rows {
		if err := b.row(changedSheet, i+2, values); err != nil {
			return err
		}
		if values[3] == "Percent complete" {
			if err := b.f.SetCellStyle(changedSheet, cell(5, i+2), cell(6, i+2), percent); err != nil {
				return err
			}
		}
	}

	return b.filter(changedSheet, len(headers), len(rows))
}

func (b *book) moved(r diff.Report) error {
	headers := []string{"Row", "Outline", "Task", "Kind", "Start before", "Start now", "Start shift, days", "Finish before", "Finish now", "Finish shift, days"}
	if err := b.sheet(movedSheet, headers, []float64{7, 10, 46, 10, 18, 18, 14, 18, 18, 14}); err != nil {
		return err
	}

	row := 2
	for _, group := range []struct {
		kind  string
		moves []diff.Move
	}{{"Task", r.Moved}, {"Summary", r.Rollups}} {
		for _, m := range group.moves {
			if err := b.row(movedSheet, row, []any{m.Task.Row, m.Task.Outline, m.Task.Name, group.kind}); err != nil {
				return err
			}
			if err := b.shift(movedSheet, row, 5, m.Start); err != nil {
				return err
			}
			if err := b.shift(movedSheet, row, 8, m.Finish); err != nil {
				return err
			}
			row++
		}
	}

	return b.filter(movedSheet, len(headers), row-2)
}

func (b *book) shift(sheet string, row, col int, s *diff.Shift) error {
	if s == nil {
		return nil
	}
	if err := b.dateCell(sheet, cell(col, row), s.Old); err != nil {
		return err
	}
	if err := b.dateCell(sheet, cell(col+1, row), s.New); err != nil {
		return err
	}
	if err := b.f.SetCellValue(sheet, cell(col+2, row), days(s)); err != nil {
		return err
	}
	return b.f.SetCellStyle(sheet, cell(col+2, row), cell(col+2, row), b.decimal)
}

func (b *book) dateCell(sheet, axis string, value *string) error {
	if value == nil {
		return nil
	}
	t, err := time.Parse(contract.DateLayout, *value)
	if err != nil {
		return b.f.SetCellStr(sheet, axis, *value)
	}
	if err = b.f.SetCellValue(sheet, axis, t); err != nil {
		return err
	}
	return b.f.SetCellStyle(sheet, axis, axis, b.date)
}

func (b *book) links(r diff.Report) error {
	headers := []string{"From row", "From task", "To row", "To task", "Before", "Now"}
	if err := b.sheet(linksSheet, headers, []float64{9, 40, 9, 40, 16, 16}); err != nil {
		return err
	}

	for i, l := range r.Links {
		values := []any{l.Predecessor.Row, l.Predecessor.Name, l.Successor.Row, l.Successor.Name, linkText(l.Old), linkText(l.New)}
		if err := b.row(linksSheet, i+2, values); err != nil {
			return err
		}
	}

	return b.filter(linksSheet, len(headers), len(r.Links))
}

func (b *book) critical(r diff.Report) error {
	headers := []string{"Row", "Outline", "Task", "Change"}
	if err := b.sheet(criticalSheet, headers, []float64{7, 10, 46, 20}); err != nil {
		return err
	}

	row := 2
	for _, group := range []struct {
		change string
		refs   []diff.Ref
	}{{"Became critical", r.Entered}, {"No longer critical", r.Left}} {
		for _, ref := range group.refs {
			if err := b.row(criticalSheet, row, []any{ref.Row, ref.Outline, ref.Name, group.change}); err != nil {
				return err
			}
			row++
		}
	}

	return b.filter(criticalSheet, len(headers), row-2)
}

func days(s *diff.Shift) float64 {
	return math.Round(float64(s.Minutes)/1440*100) / 100
}

func phase(r *diff.Ref) string {
	if r == nil {
		return "top level"
	}
	return r.Name
}

func amountText(a *contract.Amount) string {
	if a == nil {
		return "0"
	}
	return fmt.Sprintf("%g %s", a.Value, strings.ToLower(strings.ReplaceAll(a.Units, "_", " ")))
}

func flagText(on bool) string {
	if on {
		return "yes"
	}
	return "no"
}

func linkText(l *diff.Link) string {
	if l == nil {
		return "none"
	}
	text := short(l.Type)
	if l.Lag != nil && l.Lag.Value != 0 {
		text += fmt.Sprintf(" %+g %s", l.Lag.Value, strings.ToLower(strings.ReplaceAll(l.Lag.Units, "_", " ")))
	}
	return text
}
