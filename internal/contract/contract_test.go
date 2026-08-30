package contract

import (
	"server/internal/assert"
	"server/internal/fixtures"
	"testing"
)

func TestDecodeFixtures(t *testing.T) {
	for name, raw := range fixtures.All() {
		t.Run(name, func(t *testing.T) {
			c, err := Decode(raw)
			assert.NilError(t, err)

			if len(c.Tasks) == 0 {
				t.Fatal("no tasks decoded")
			}

			ids := make(map[int64]bool, len(c.Tasks))
			for _, task := range c.Tasks {
				if ids[task.ID] {
					t.Errorf("duplicate task id %d", task.ID)
				}
				ids[task.ID] = true
			}

			for _, task := range c.Tasks {
				if task.ParentID != nil && !ids[*task.ParentID] {
					t.Errorf("task %d points at missing parent %d", task.ID, *task.ParentID)
				}
			}

			names := c.ResourceNames()
			for _, task := range c.Tasks {
				for _, a := range task.Assignments {
					if _, ok := names[a.ResourceID]; !ok {
						t.Errorf("task %d assigned to missing resource %d", task.ID, a.ResourceID)
					}
				}
			}

			for _, rel := range c.Relations {
				if !ids[rel.PredecessorID] || !ids[rel.SuccessorID] {
					t.Errorf("relation %d links missing tasks %d → %d",
						rel.ID, rel.PredecessorID, rel.SuccessorID)
				}
			}
		})
	}
}

func TestDecodeRejectsUnknownField(t *testing.T) {
	_, err := Decode([]byte(`{"contract_version":1,"surprise":true}`))
	if err == nil {
		t.Fatal("unknown field accepted; drift from the parser would go unnoticed")
	}
}

func TestDecodeRejectsOtherVersion(t *testing.T) {
	_, err := Decode([]byte(`{"contract_version":2}`))
	if err == nil {
		t.Fatal("wrong contract version accepted")
	}
}

func TestDecodeRejectsHostileStrings(t *testing.T) {
	tests := map[string]string{
		"task start":         `"start":"<img src=x onerror=alert(1)>"`,
		"task finish":        `"finish":"2026-01-05"`,
		"baseline start":     `"baseline":{"start":"2026-01-05T08:00","finish":null}`,
		"duration units":     `"duration":{"value":1.0,"units":"<b>d</b>"}`,
		"zoned date":         `"start":"2026-01-05T08:00:00Z"`,
		"space instead of T": `"start":"2026-01-05 08:00:00"`,
	}

	for name, field := range tests {
		t.Run(name, func(t *testing.T) {
			raw := `{"contract_version":1,` +
				`"project":{"name":null,"start":null,"finish":null},` +
				`"calendar":{"name":null,"non_working_weekdays":[],"exceptions":[]},` +
				`"resources":[],` +
				`"tasks":[{"id":1,"parent_id":null,"name":"x","wbs":null,"outline_number":null,` +
				`"outline_level":1,"start":null,"finish":null,"duration":null,"percent_complete":0.0,` +
				`"is_summary":false,"is_milestone":false,"is_critical":false,"notes":null,` +
				`"baseline":null,"assignments":[],` + field + `}],` +
				`"relations":[]}`

			if _, err := Decode([]byte(raw)); err == nil {
				t.Fatalf("accepted hostile %s", name)
			}
		})
	}
}

func TestDecodeRejectsHostileRelationType(t *testing.T) {
	raw := `{"contract_version":1,` +
		`"project":{"name":null,"start":null,"finish":null},` +
		`"calendar":{"name":null,"non_working_weekdays":[],"exceptions":[]},` +
		`"resources":[],"tasks":[],` +
		`"relations":[{"id":1,"predecessor_id":1,"successor_id":2,` +
		`"type":"<script>alert(1)</script>","lag":null}]}`

	if _, err := Decode([]byte(raw)); err == nil {
		t.Fatal("accepted hostile relation type")
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	document := `{"contract_version":1,` +
		`"project":{"name":null,"start":null,"finish":null},` +
		`"calendar":{"name":null,"non_working_weekdays":[],"exceptions":[]},` +
		`"resources":[],"tasks":[],"relations":[]}`

	if _, err := Decode([]byte(document + "\n")); err != nil {
		t.Fatalf("rejected a trailing newline: %s", err)
	}

	rejected := map[string]string{
		"markup":          document + "<script>alert(1)</script>",
		"second document": document + document,
		"garbage":         document + "!!!",
		"json fragment":   document + `,"tasks":[]`,
	}

	for name, raw := range rejected {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(raw)); err == nil {
				t.Fatalf("accepted trailing %s", name)
			}
		})
	}
}
