package xlsx

import (
	"io"
	"server/internal/contract"
	"server/internal/lookahead"

	"github.com/xuri/excelize/v2"
)

const (
	lookaheadSheet = "Lookahead"
	resourceSheet  = "By resource"
	noResource     = "No resource"
)

var resourceHeaders = []string{"Resource", "ID", "Task", "Phase", "Start", "Finish", "Complete", "Critical"}

var resourceWidths = []float64{26, 6, 46, 30, 18, 18, 10, 9}

const (
	resourceStartColumn    = 5
	resourceFinishColumn   = 6
	resourceCompleteColumn = 7
)

func WriteLookahead(w io.Writer, c *contract.Contract, window lookahead.Window) error {
	f := excelize.NewFile()
	defer f.Close()

	s, err := newStyles(f)
	if err != nil {
		return err
	}

	if err = addSheet(f, lookaheadSheet, true); err != nil {
		return err
	}
	if err = writeTasks(f, lookaheadSheet, c, window.Select(c), s); err != nil {
		return err
	}

	groups := byResource(c, window)
	if len(groups) > 0 {
		if err = addSheet(f, resourceSheet, false); err != nil {
			return err
		}
		if err = writeResources(f, c, groups, s); err != nil {
			return err
		}
	}

	return f.Write(w)
}

type group struct {
	name  string
	tasks []contract.Task
}

func byResource(c *contract.Contract, window lookahead.Window) []group {
	if len(c.Resources) == 0 {
		return nil
	}

	names := c.ResourceNames()
	tasks := map[int64][]contract.Task{}
	var unassigned []contract.Task
	assignedAny := false

	for _, t := range c.Tasks {
		if !window.Holds(t) {
			continue
		}

		placed := false
		seen := map[int64]bool{}
		for _, a := range t.Assignments {
			if names[a.ResourceID] == "" || seen[a.ResourceID] {
				continue
			}
			seen[a.ResourceID] = true
			tasks[a.ResourceID] = append(tasks[a.ResourceID], t)
			placed = true
			assignedAny = true
		}
		if !placed {
			unassigned = append(unassigned, t)
		}
	}

	if !assignedAny {
		return nil
	}

	var out []group
	for _, r := range c.Resources {
		if list := tasks[r.ID]; len(list) > 0 {
			out = append(out, group{name: names[r.ID], tasks: list})
		}
	}
	if len(unassigned) > 0 {
		out = append(out, group{name: noResource, tasks: unassigned})
	}

	return out
}

func writeResources(f *excelize.File, c *contract.Contract, groups []group, s styles) error {
	if err := header(f, resourceSheet, resourceHeaders, resourceWidths, s); err != nil {
		return err
	}

	rowIDs := c.RowIDs()
	phases := phaseNames(c)

	row := 1
	for _, g := range groups {
		for _, t := range g.tasks {
			row++

			values := []any{
				g.name,
				rowIDs[t.ID],
				deref(t.Name),
				phases[t.ID],
				nil, nil,
				t.PercentComplete / 100,
				flag(t.IsCritical),
			}
			if err := f.SetSheetRow(resourceSheet, cell(1, row), &values); err != nil {
				return err
			}
			if err := setDate(f, resourceSheet, cell(resourceStartColumn, row), t.Start, s.date); err != nil {
				return err
			}
			if err := setDate(f, resourceSheet, cell(resourceFinishColumn, row), t.Finish, s.date); err != nil {
				return err
			}
		}
	}

	if row > 1 {
		if err := f.SetCellStyle(resourceSheet, cell(resourceCompleteColumn, 2), cell(resourceCompleteColumn, row), s.percent); err != nil {
			return err
		}
	}

	return finish(f, resourceSheet, len(resourceHeaders), row-1)
}

func phaseNames(c *contract.Contract) map[int64]string {
	byID := make(map[int64]contract.Task, len(c.Tasks))
	for _, t := range c.Tasks {
		byID[t.ID] = t
	}

	out := make(map[int64]string, len(c.Tasks))
	for _, t := range c.Tasks {
		if t.ParentID == nil {
			continue
		}
		if parent, ok := byID[*t.ParentID]; ok && parent.OutlineLevel > 0 {
			out[t.ID] = deref(parent.Name)
		}
	}
	return out
}
