package xlsx

import (
	"fmt"
	"io"
	"server/internal/contract"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	sheet                  = "Tasks"
	inputTime              = "2006-01-02T15:04:05"
	maxVisibleOutlineLevel = 10
)

var headers = []string{
	"ID", "WBS", "Task", "Start", "Finish", "Duration", "Units",
	"Complete", "Critical", "Milestone", "Resources", "Predecessors", "Notes",
}

var widths = []float64{6, 10, 46, 18, 18, 10, 10, 10, 9, 10, 28, 22, 40}

const (
	startColumn    = 4
	finishColumn   = 5
	completeColumn = 8
)

type styles struct {
	head        int
	date        int
	summary     int
	summaryDate int
	percent     int
}

func newStyles(f *excelize.File) (styles, error) {
	var s styles
	var err error

	if s.head, err = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F7F8FA"}},
	}); err != nil {
		return s, err
	}
	if s.date, err = f.NewStyle(&excelize.Style{NumFmt: 22}); err != nil {
		return s, err
	}
	if s.summary, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}}); err != nil {
		return s, err
	}
	if s.summaryDate, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, NumFmt: 22}); err != nil {
		return s, err
	}
	if s.percent, err = f.NewStyle(&excelize.Style{NumFmt: 9}); err != nil {
		return s, err
	}

	return s, nil
}

func Write(w io.Writer, c *contract.Contract) error {
	f := excelize.NewFile()
	defer f.Close()

	s, err := newStyles(f)
	if err != nil {
		return err
	}

	if err = addSheet(f, sheet, true); err != nil {
		return err
	}
	if err = writeTasks(f, sheet, c, nil, s); err != nil {
		return err
	}

	return f.Write(w)
}

func addSheet(f *excelize.File, name string, first bool) error {
	index, err := f.NewSheet(name)
	if err != nil {
		return err
	}
	if first {
		f.SetActiveSheet(index)
		return f.DeleteSheet("Sheet1")
	}
	return nil
}

func header(f *excelize.File, name string, columns []string, sizes []float64, s styles) error {
	if err := f.SetSheetRow(name, "A1", &columns); err != nil {
		return err
	}
	if err := f.SetCellStyle(name, "A1", cell(len(columns), 1), s.head); err != nil {
		return err
	}

	for i, width := range sizes {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetColWidth(name, col, col, width); err != nil {
			return err
		}
	}

	return nil
}

func finish(f *excelize.File, name string, columns, rows int) error {
	if err := f.AutoFilter(name, fmt.Sprintf("A1:%s", cell(columns, rows+1)), nil); err != nil {
		return err
	}
	return f.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

func writeTasks(f *excelize.File, name string, c *contract.Contract, keep map[int64]bool, s styles) error {
	if err := header(f, name, headers, widths, s); err != nil {
		return err
	}

	names := c.ResourceNames()
	predecessors := c.PredecessorsOf()
	rowIDs := c.RowIDs()

	row := 1
	for _, task := range c.Tasks {
		if keep != nil && !keep[task.ID] {
			continue
		}
		row++

		values := []any{
			rowIDs[task.ID],
			text(task.OutlineNumber, task.WBS),
			indent(task),
			nil, nil,
			amount(task.Duration),
			units(task.Duration),
			task.PercentComplete / 100,
			flag(task.IsCritical),
			flag(task.IsMilestone),
			assigned(task, names),
			predecessorList(predecessors[task.ID], rowIDs),
			deref(task.Notes),
		}

		if err := f.SetSheetRow(name, cell(1, row), &values); err != nil {
			return err
		}
		dateStyle := s.date
		if task.IsSummary {
			dateStyle = s.summaryDate
			if err := f.SetCellStyle(name, cell(1, row), cell(len(headers), row), s.summary); err != nil {
				return err
			}
		}

		if err := setDate(f, name, cell(startColumn, row), task.Start, dateStyle); err != nil {
			return err
		}
		if err := setDate(f, name, cell(finishColumn, row), task.Finish, dateStyle); err != nil {
			return err
		}
	}

	if row > 1 {
		if err := f.SetCellStyle(name, cell(completeColumn, 2), cell(completeColumn, row), s.percent); err != nil {
			return err
		}
	}

	return finish(f, name, len(headers), row-1)
}

func setDate(f *excelize.File, name, axis string, value *string, style int) error {
	if value == nil {
		return nil
	}

	t, err := time.Parse(inputTime, *value)
	if err != nil {
		return f.SetCellStr(name, axis, *value)
	}

	if err = f.SetCellValue(name, axis, t); err != nil {
		return err
	}

	return f.SetCellStyle(name, axis, axis, style)
}

func cell(col, row int) string {
	axis, _ := excelize.CoordinatesToCellName(col, row)
	return axis
}

func indent(t contract.Task) string {
	name := deref(t.Name)
	level := min(t.OutlineLevel, maxVisibleOutlineLevel)
	if level > 1 {
		return strings.Repeat("    ", level-1) + name
	}
	return name
}

func text(values ...*string) string {
	for _, v := range values {
		if v != nil && *v != "" {
			return *v
		}
	}
	return ""
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func amount(a *contract.Amount) any {
	if a == nil {
		return nil
	}
	return a.Value
}

func units(a *contract.Amount) string {
	if a == nil {
		return ""
	}
	return strings.ToLower(a.Units)
}

func flag(on bool) string {
	if on {
		return "yes"
	}
	return ""
}

func assigned(t contract.Task, names map[int64]string) string {
	if len(t.Assignments) == 0 {
		return ""
	}

	parts := make([]string, 0, len(t.Assignments))
	for _, a := range t.Assignments {
		name := names[a.ResourceID]
		if name == "" {
			continue
		}
		if a.Units != nil && *a.Units != 100 {
			name = fmt.Sprintf("%s (%g%%)", name, *a.Units)
		}
		parts = append(parts, name)
	}

	return strings.Join(parts, ", ")
}

func predecessorList(relations []contract.Relation, rowIDs map[int64]int) string {
	if len(relations) == 0 {
		return ""
	}

	parts := make([]string, 0, len(relations))
	for _, r := range relations {
		rowID, ok := rowIDs[r.PredecessorID]
		if !ok {
			continue
		}

		part := fmt.Sprintf("%d", rowID)
		if r.Type != "FINISH_START" {
			part += " " + short(r.Type)
		}
		if r.Lag != nil && r.Lag.Value != 0 {
			part += fmt.Sprintf("%+g%s", r.Lag.Value, strings.ToLower(r.Lag.Units)[:1])
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, ", ")
}

func short(kind string) string {
	switch kind {
	case "START_START":
		return "SS"
	case "FINISH_FINISH":
		return "FF"
	case "START_FINISH":
		return "SF"
	default:
		return "FS"
	}
}
