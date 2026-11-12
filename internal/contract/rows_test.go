package contract

import (
	"server/internal/assert"
	"testing"
)

func tasksWithIDs(ids ...int64) *Contract {
	c := &Contract{}
	for _, id := range ids {
		c.Tasks = append(c.Tasks, Task{ID: id})
	}
	return c
}

func TestRowIDsCountFromTheProjectSummaryRow(t *testing.T) {
	rows := tasksWithIDs(0, 1, 2, 3, 4, 7, 6, 5).RowIDs()

	want := map[int64]int{0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 7: 5, 6: 6, 5: 7}
	for uniqueID, row := range want {
		assert.Equal(t, rows[uniqueID], row)
	}
}

func TestRowIDsStartAtOneWithoutAProjectSummaryRow(t *testing.T) {
	rows := tasksWithIDs(1, 2, 19, 3).RowIDs()

	want := map[int64]int{1: 1, 2: 2, 19: 3, 3: 4}
	for uniqueID, row := range want {
		assert.Equal(t, rows[uniqueID], row)
	}
}

func TestARepeatedTaskIDKeepsItsFirstRow(t *testing.T) {
	rows := tasksWithIDs(0, 5, 5).RowIDs()

	assert.Equal(t, rows[5], 1)
}

func TestAnEmptyPlanHasNoRows(t *testing.T) {
	assert.Equal(t, len(tasksWithIDs().RowIDs()), 0)
}
