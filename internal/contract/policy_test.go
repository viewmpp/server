package contract

import (
	"bytes"
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
			c.Relations = make([]Relation, MaxRelationsPerTask+1)
			for i := range c.Relations {
				c.Relations[i] = Relation{ID: int64(i + 1), PredecessorID: int64(i), SuccessorID: 1, Type: "FINISH_START"}
			}
		},
		"successors per task": func(c *Contract) {
			c.Relations = make([]Relation, MaxRelationsPerTask+1)
			for i := range c.Relations {
				c.Relations[i] = Relation{ID: int64(i + 1), PredecessorID: 1, SuccessorID: int64(i), Type: "FINISH_START"}
			}
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
