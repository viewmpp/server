package contract

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestDecodeEnforcesContractByteLimit(t *testing.T) {
	document := []byte(`{"contract_version":1,"project":{},"calendar":{},"resources":[],"tasks":[],"relations":[]}`)
	atLimit := append(document, bytes.Repeat([]byte{' '}, MaxBytes-len(document))...)
	if _, err := Decode(atLimit); err != nil {
		t.Fatalf("rejected contract at byte limit: %v", err)
	}

	_, err := Decode(append(atLimit, ' '))
	if err == nil {
		t.Fatal("accepted contract over byte limit")
	}
	if !errors.Is(err, ErrLimit) {
		t.Errorf("an oversized contract is not reported as a limit: %v", err)
	}
}

func TestPolicyAcceptsBoundaryValues(t *testing.T) {
	text := strings.Repeat("界", MaxTextRunes)
	notes := strings.Repeat("界", MaxNotesRunes)
	units := float64(MaxAssignmentUnits)
	parentID := int64(MaxID)

	c := Contract{
		Project:   Project{Name: &text},
		Calendar:  Calendar{Name: &text},
		Resources: []Resource{{ID: MaxID, Name: text}},
		Tasks: []Task{{
			ID:              MaxID,
			ParentID:        &parentID,
			Name:            &text,
			WBS:             &text,
			OutlineNumber:   &text,
			OutlineLevel:    MaxOutlineLevel,
			Duration:        &Amount{Value: MaxNumber, Units: "MINUTES"},
			PercentComplete: 100,
			Notes:           &notes,
			Assignments:     []Assignment{{ResourceID: MaxID, Units: &units}},
		}},
		Relations: []Relation{{
			ID:            MaxID,
			PredecessorID: MaxID,
			SuccessorID:   MaxID,
			Type:          "FINISH_START",
			Lag:           &Amount{Value: -MaxNumber, Units: "MINUTES"},
		}},
	}

	if err := c.validate(); err != nil {
		t.Fatalf("boundary values rejected: %v", err)
	}
}

func TestPolicyRejectsCollectionLimits(t *testing.T) {
	tests := map[string]func(*Contract){
		"weekdays": func(c *Contract) {
			c.Calendar.NonWorkingWeekdays = make([]string, 8)
		},
		"exceptions": func(c *Contract) {
			c.Calendar.Exceptions = make([]CalendarException, MaxCalendarExceptions+1)
		},
		"resources": func(c *Contract) {
			c.Resources = make([]Resource, MaxResources+1)
		},
		"tasks": func(c *Contract) {
			c.Tasks = make([]Task, MaxTasks+1)
		},
		"relations": func(c *Contract) {
			c.Relations = make([]Relation, MaxRelations+1)
		},
		"assignments per task": func(c *Contract) {
			c.Tasks = []Task{{Assignments: make([]Assignment, MaxAssignmentsPerTask+1)}}
		},
		"assignments total": func(c *Contract) {
			c.Tasks = make([]Task, MaxAssignments/MaxAssignmentsPerTask+1)
			for i := range c.Tasks {
				c.Tasks[i].Assignments = make([]Assignment, MaxAssignmentsPerTask)
			}
		},
		"predecessors per task": func(c *Contract) {
			c.Relations = fanIn(MaxPredecessorsPerTask + 1)
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var c Contract
			mutate(&c)
			if err := c.validate(); err == nil {
				t.Fatal("accepted collection over limit")
			}
		})
	}
}

func fanIn(n int) []Relation {
	relations := make([]Relation, n)
	for i := range relations {
		relations[i] = Relation{ID: int64(i + 1), PredecessorID: int64(i + 2), SuccessorID: 1, Type: "FINISH_START"}
	}
	return relations
}

func fanOut(n int) []Relation {
	relations := make([]Relation, n)
	for i := range relations {
		relations[i] = Relation{ID: int64(i + 1), PredecessorID: 1, SuccessorID: int64(i + 2), Type: "FINISH_START"}
	}
	return relations
}

func TestPolicyAcceptsPredecessorsUpToTheLimit(t *testing.T) {
	c := Contract{Relations: fanIn(MaxPredecessorsPerTask)}

	if err := c.validate(); err != nil {
		t.Fatalf("%d predecessors rejected: %v", MaxPredecessorsPerTask, err)
	}
}

func TestPolicyDoesNotLimitSuccessorsPerTask(t *testing.T) {
	c := Contract{Relations: fanOut(1000)}

	if err := c.validate(); err != nil {
		t.Fatalf("a task with 1000 successors was rejected: %v", err)
	}
}

func TestLimitErrorsAreTellableFromMalformedContracts(t *testing.T) {
	long := strings.Repeat("a", MaxTextRunes+1)

	tests := []struct {
		name   string
		mutate func(*Contract)
		limit  bool
	}{
		{"too many tasks", func(c *Contract) { c.Tasks = make([]Task, MaxTasks+1) }, true},
		{"too many predecessors", func(c *Contract) { c.Relations = fanIn(MaxPredecessorsPerTask + 1) }, true},
		{"task name too long", func(c *Contract) { c.Tasks = []Task{{Name: &long}} }, true},
		{"negative id", func(c *Contract) { c.Tasks = []Task{{ID: -1}} }, false},
		{"completion over 100", func(c *Contract) { c.Tasks = []Task{{PercentComplete: 101}} }, false},
		{"identifier too long", func(c *Contract) {
			c.Relations = []Relation{{ID: 1, Type: strings.Repeat("A", MaxTokenRunes+1)}}
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Contract
			tt.mutate(&c)

			err := c.validate()
			if err == nil {
				t.Fatal("accepted an invalid contract")
			}
			if got := errors.Is(err, ErrLimit); got != tt.limit {
				t.Errorf("errors.Is(err, ErrLimit) = %v, want %v: %v", got, tt.limit, err)
			}
		})
	}
}

func TestPolicyRejectsStringLimits(t *testing.T) {
	tests := map[string]func(*Contract){
		"text": func(c *Contract) {
			value := strings.Repeat("界", MaxTextRunes+1)
			c.Project.Name = &value
		},
		"notes": func(c *Contract) {
			value := strings.Repeat("界", MaxNotesRunes+1)
			c.Tasks = []Task{{Notes: &value}}
		},
		"token": func(c *Contract) {
			c.Relations = []Relation{{ID: 1, Type: strings.Repeat("A", MaxTokenRunes+1)}}
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var c Contract
			mutate(&c)
			if err := c.validate(); err == nil {
				t.Fatal("accepted string over limit")
			}
		})
	}
}

func TestPolicyRejectsNumericRanges(t *testing.T) {
	tests := map[string]func(*Contract){
		"negative id": func(c *Contract) {
			c.Tasks = []Task{{ID: -1}}
		},
		"large id": func(c *Contract) {
			c.Tasks = []Task{{ID: MaxID + 1}}
		},
		"negative outline": func(c *Contract) {
			c.Tasks = []Task{{OutlineLevel: -1}}
		},
		"large outline": func(c *Contract) {
			c.Tasks = []Task{{OutlineLevel: MaxOutlineLevel + 1}}
		},
		"negative completion": func(c *Contract) {
			c.Tasks = []Task{{PercentComplete: -1}}
		},
		"large completion": func(c *Contract) {
			c.Tasks = []Task{{PercentComplete: 101}}
		},
		"negative duration": func(c *Contract) {
			c.Tasks = []Task{{Duration: &Amount{Value: -1, Units: "MINUTES"}}}
		},
		"large duration": func(c *Contract) {
			c.Tasks = []Task{{Duration: &Amount{Value: MaxNumber + 1, Units: "MINUTES"}}}
		},
		"large assignment units": func(c *Contract) {
			units := float64(MaxAssignmentUnits + 1)
			c.Tasks = []Task{{Assignments: []Assignment{{Units: &units}}}}
		},
		"negative assignment units": func(c *Contract) {
			units := -1.0
			c.Tasks = []Task{{Assignments: []Assignment{{Units: &units}}}}
		},
		"not a number completion": func(c *Contract) {
			c.Tasks = []Task{{PercentComplete: math.NaN()}}
		},
		"infinite lag": func(c *Contract) {
			c.Relations = []Relation{{ID: 1, Type: "FINISH_START", Lag: &Amount{Value: math.Inf(1), Units: "MINUTES"}}}
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var c Contract
			mutate(&c)
			if err := c.validate(); err == nil {
				t.Fatal("accepted number outside range")
			}
		})
	}
}
