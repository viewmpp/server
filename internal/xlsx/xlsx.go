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
	sheet     = "Tasks"
	inputTime = "2006-01-02T15:04:05"
)

var headers = []string{
	"WBS", "Task", "Start", "Finish", "Duration", "Units",
	"Complete", "Critical", "Milestone", "Resources", "Predecessors", "Notes",
}

var widths = []float64{10, 46, 18, 18, 10, 10, 10, 9, 10, 28, 22, 40}

func Write(w io.Writer, c *contract.Contract) error {
	f := excelize.NewFile()
	defer f.Close()

	index, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	head, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F7F8FA"}},
	})
	if err != nil {
		return err
	}

	date, err := f.NewStyle(&excelize.Style{NumFmt: 22})
	if err != nil {
		return err
	}

	summary, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}

	if err = f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}
	if err = f.SetCellStyle(sheet, "A1", cell(len(headers), 1), head); err != nil {
		return err
	}

	for i, width := range widths {
		name, _ := excelize.ColumnNumberToName(i + 1)
		if err = f.SetColWidth(sheet, name, name, width); err != nil {
			return err
		}
	}

	names := c.ResourceNames()
	predecessors := c.PredecessorsOf()

	for i, task := range c.Tasks {
		row := i + 2

		values := []any{
			text(task.WBS, task.OutlineNumber),
			indent(task),
			nil, nil,
			amount(task.Duration),
			units(task.Duration),
			task.PercentComplete / 100,
			flag(task.IsCritical),
			flag(task.IsMilestone),
			assigned(task, names),
			predecessorList(predecessors[task.ID]),
			deref(task.Notes),
		}

		if err = f.SetSheetRow(sheet, cell(1, row), &values); err != nil {
			return err
		}

		if err = setDate(f, cell(3, row), task.Start, date); err != nil {
			return err
		}
		if err = setDate(f, cell(4, row), task.Finish, date); err != nil {
			return err
		}

		if task.IsSummary {
			if err = f.SetCellStyle(sheet, cell(1, row), cell(len(headers), row), summary); err != nil {
				return err
			}
		}
	}

	percent, err := f.NewStyle(&excelize.Style{NumFmt: 9})
	if err != nil {
		return err
	}
	if err = f.SetCellStyle(sheet, "G2", cell(7, len(c.Tasks)+1), percent); err != nil {
		return err
	}

	if err = f.AutoFilter(sheet, fmt.Sprintf("A1:%s", cell(len(headers), len(c.Tasks)+1)), nil); err != nil {
		return err
	}
	if err = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}

	return f.Write(w)
}

func setDate(f *excelize.File, axis string, value *string, style int) error {
	if value == nil {
		return nil
	}

	t, err := time.Parse(inputTime, *value)
	if err != nil {
		return f.SetCellStr(sheet, axis, *value)
	}

	if err = f.SetCellValue(sheet, axis, t); err != nil {
		return err
	}

	return f.SetCellStyle(sheet, axis, axis, style)
}

func cell(col, row int) string {
	axis, _ := excelize.CoordinatesToCellName(col, row)
	return axis
}

func indent(t contract.Task) string {
	name := deref(t.Name)
	if t.OutlineLevel > 1 {
		return strings.Repeat("    ", t.OutlineLevel-1) + name
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

func predecessorList(relations []contract.Relation) string {
	if len(relations) == 0 {
		return ""
	}

	parts := make([]string, 0, len(relations))
	for _, r := range relations {
		part := fmt.Sprintf("%d", r.PredecessorID)
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
