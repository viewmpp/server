package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const Version = 1

const DateLayout = "2006-01-02T15:04:05"

type Contract struct {
	Version   int        `json:"contract_version"`
	Project   Project    `json:"project"`
	Calendar  Calendar   `json:"calendar"`
	Resources []Resource `json:"resources"`
	Tasks     []Task     `json:"tasks"`
	Relations []Relation `json:"relations"`
}

type Project struct {
	Name   *string `json:"name"`
	Start  *string `json:"start"`
	Finish *string `json:"finish"`
}

type Calendar struct {
	Name               *string             `json:"name"`
	NonWorkingWeekdays []string            `json:"non_working_weekdays"`
	Exceptions         []CalendarException `json:"exceptions"`
}

type CalendarException struct {
	From    *string `json:"from"`
	To      *string `json:"to"`
	Working bool    `json:"working"`
	Name    *string `json:"name"`
}

type Resource struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Task struct {
	ID              int64        `json:"id"`
	ParentID        *int64       `json:"parent_id"`
	Name            *string      `json:"name"`
	WBS             *string      `json:"wbs"`
	OutlineNumber   *string      `json:"outline_number"`
	OutlineLevel    int          `json:"outline_level"`
	Start           *string      `json:"start"`
	Finish          *string      `json:"finish"`
	Duration        *Amount      `json:"duration"`
	PercentComplete float64      `json:"percent_complete"`
	IsSummary       bool         `json:"is_summary"`
	IsMilestone     bool         `json:"is_milestone"`
	IsCritical      bool         `json:"is_critical"`
	Notes           *string      `json:"notes"`
	Baseline        *Baseline    `json:"baseline"`
	Assignments     []Assignment `json:"assignments"`
}

type Baseline struct {
	Start  *string `json:"start"`
	Finish *string `json:"finish"`
}

type Assignment struct {
	ResourceID int64    `json:"resource_id"`
	Units      *float64 `json:"units"`
}

type Amount struct {
	Value float64 `json:"value"`
	Units string  `json:"units"`
}

type Relation struct {
	ID            int64   `json:"id"`
	PredecessorID int64   `json:"predecessor_id"`
	SuccessorID   int64   `json:"successor_id"`
	Type          string  `json:"type"`
	Lag           *Amount `json:"lag"`
}

func Decode(data []byte) (*Contract, error) {
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("contract is %d bytes, limit is %d", len(data), MaxBytes)
	}

	var c Contract

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("decode contract: %w", err)
	}

	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("contract is followed by more data")
	}

	if c.Version != Version {
		return nil, fmt.Errorf("contract version %d, want %d", c.Version, Version)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Contract) validate() error {
	if err := checkCount("calendar.non_working_weekdays", len(c.Calendar.NonWorkingWeekdays), 7); err != nil {
		return err
	}
	if err := checkCount("calendar.exceptions", len(c.Calendar.Exceptions), MaxCalendarExceptions); err != nil {
		return err
	}
	if err := checkCount("resources", len(c.Resources), MaxResources); err != nil {
		return err
	}
	if err := checkCount("tasks", len(c.Tasks), MaxTasks); err != nil {
		return err
	}
	if err := checkCount("relations", len(c.Relations), MaxRelations); err != nil {
		return err
	}

	if err := checkString("project.name", c.Project.Name, MaxTextRunes); err != nil {
		return err
	}
	if err := checkDate("project.start", c.Project.Start); err != nil {
		return err
	}
	if err := checkDate("project.finish", c.Project.Finish); err != nil {
		return err
	}

	if err := checkString("calendar.name", c.Calendar.Name, MaxTextRunes); err != nil {
		return err
	}

	for i, day := range c.Calendar.NonWorkingWeekdays {
		if err := checkToken(fmt.Sprintf("calendar.non_working_weekdays[%d]", i), day); err != nil {
			return err
		}
	}

	for i, ex := range c.Calendar.Exceptions {
		if err := checkDate(fmt.Sprintf("calendar.exceptions[%d].from", i), ex.From); err != nil {
			return err
		}
		if err := checkDate(fmt.Sprintf("calendar.exceptions[%d].to", i), ex.To); err != nil {
			return err
		}
		if err := checkString(fmt.Sprintf("calendar.exceptions[%d].name", i), ex.Name, MaxTextRunes); err != nil {
			return err
		}
	}

	for i, resource := range c.Resources {
		where := fmt.Sprintf("resources[%d]", i)
		if err := checkInteger(where+".id", resource.ID, 0); err != nil {
			return err
		}
		if err := checkRequiredString(where+".name", resource.Name, MaxTextRunes); err != nil {
			return err
		}
	}

	assignments := 0

	for i, task := range c.Tasks {
		where := fmt.Sprintf("tasks[%d]", i)

		if err := checkInteger(where+".id", task.ID, 0); err != nil {
			return err
		}
		if task.ParentID != nil {
			if err := checkInteger(where+".parent_id", *task.ParentID, 0); err != nil {
				return err
			}
		}
		if err := checkString(where+".name", task.Name, MaxTextRunes); err != nil {
			return err
		}
		if err := checkString(where+".wbs", task.WBS, MaxTextRunes); err != nil {
			return err
		}
		if err := checkString(where+".outline_number", task.OutlineNumber, MaxTextRunes); err != nil {
			return err
		}
		if task.OutlineLevel < 0 || task.OutlineLevel > MaxOutlineLevel {
			return fmt.Errorf("%s.outline_level is %d, range is 0..%d", where, task.OutlineLevel, MaxOutlineLevel)
		}
		if err := checkDate(where+".start", task.Start); err != nil {
			return err
		}
		if err := checkDate(where+".finish", task.Finish); err != nil {
			return err
		}
		if err := checkAmount(where+".duration", task.Duration, 0, MaxNumber); err != nil {
			return err
		}
		if err := checkNumber(where+".percent_complete", task.PercentComplete, 0, 100); err != nil {
			return err
		}
		if err := checkString(where+".notes", task.Notes, MaxNotesRunes); err != nil {
			return err
		}
		if task.Baseline != nil {
			if err := checkDate(where+".baseline.start", task.Baseline.Start); err != nil {
				return err
			}
			if err := checkDate(where+".baseline.finish", task.Baseline.Finish); err != nil {
				return err
			}
		}
		if err := checkCount(where+".assignments", len(task.Assignments), MaxAssignmentsPerTask); err != nil {
			return err
		}
		assignments += len(task.Assignments)
		if assignments > MaxAssignments {
			return fmt.Errorf("assignments contains %d items, limit is %d", assignments, MaxAssignments)
		}
		for j, assignment := range task.Assignments {
			assignmentWhere := fmt.Sprintf("%s.assignments[%d]", where, j)
			if err := checkInteger(assignmentWhere+".resource_id", assignment.ResourceID, 0); err != nil {
				return err
			}
			if assignment.Units != nil {
				if err := checkNumber(assignmentWhere+".units", *assignment.Units, 0, MaxAssignmentUnits); err != nil {
					return err
				}
			}
		}
	}

	predecessors := make(map[int64]int)
	successors := make(map[int64]int)

	for i, rel := range c.Relations {
		where := fmt.Sprintf("relations[%d]", i)

		if err := checkInteger(where+".id", rel.ID, 1); err != nil {
			return err
		}
		if err := checkInteger(where+".predecessor_id", rel.PredecessorID, 0); err != nil {
			return err
		}
		if err := checkInteger(where+".successor_id", rel.SuccessorID, 0); err != nil {
			return err
		}
		if err := checkToken(where+".type", rel.Type); err != nil {
			return err
		}
		if err := checkAmount(where+".lag", rel.Lag, -MaxNumber, MaxNumber); err != nil {
			return err
		}

		predecessors[rel.SuccessorID]++
		if predecessors[rel.SuccessorID] > MaxRelationsPerTask {
			return fmt.Errorf("task %d has %d predecessors, limit is %d", rel.SuccessorID, predecessors[rel.SuccessorID], MaxRelationsPerTask)
		}
		successors[rel.PredecessorID]++
		if successors[rel.PredecessorID] > MaxRelationsPerTask {
			return fmt.Errorf("task %d has %d successors, limit is %d", rel.PredecessorID, successors[rel.PredecessorID], MaxRelationsPerTask)
		}
	}

	return nil
}

func checkDate(field string, value *string) error {
	if value == nil {
		return nil
	}

	if _, err := time.Parse(DateLayout, *value); err != nil {
		return fmt.Errorf("%s: %q is not %s", field, *value, DateLayout)
	}

	return nil
}

func checkAmount(field string, amount *Amount, min, max float64) error {
	if amount == nil {
		return nil
	}

	if err := checkNumber(field+".value", amount.Value, min, max); err != nil {
		return err
	}
	return checkToken(field+".units", amount.Units)
}

func checkToken(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is empty", field)
	}
	if err := checkRequiredString(field, value, MaxTokenRunes); err != nil {
		return err
	}

	for _, r := range value {
		if r != '_' && (r < 'A' || r > 'Z') {
			return fmt.Errorf("%s: %q is not an uppercase identifier", field, value)
		}
	}

	return nil
}

func (c *Contract) ResourceNames() map[int64]string {
	names := make(map[int64]string, len(c.Resources))
	for _, r := range c.Resources {
		names[r.ID] = r.Name
	}
	return names
}

func (c *Contract) PredecessorsOf() map[int64][]Relation {
	byTask := make(map[int64][]Relation, len(c.Relations))
	for _, rel := range c.Relations {
		byTask[rel.SuccessorID] = append(byTask[rel.SuccessorID], rel)
	}
	return byTask
}
