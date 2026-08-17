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
