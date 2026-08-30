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
	if err := checkDate("project.start", c.Project.Start); err != nil {
		return err
	}
	if err := checkDate("project.finish", c.Project.Finish); err != nil {
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
	}

	for i, task := range c.Tasks {
		where := fmt.Sprintf("tasks[%d]", i)

		if err := checkDate(where+".start", task.Start); err != nil {
			return err
		}
		if err := checkDate(where+".finish", task.Finish); err != nil {
			return err
		}
		if err := checkAmount(where+".duration", task.Duration); err != nil {
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
	}

	for i, rel := range c.Relations {
		where := fmt.Sprintf("relations[%d]", i)

		if err := checkToken(where+".type", rel.Type); err != nil {
			return err
		}
		if err := checkAmount(where+".lag", rel.Lag); err != nil {
			return err
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

func checkAmount(field string, amount *Amount) error {
	if amount == nil {
		return nil
	}

	return checkToken(field+".units", amount.Units)
}

func checkToken(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is empty", field)
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
