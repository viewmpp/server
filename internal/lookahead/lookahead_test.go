package lookahead

import (
	"server/internal/contract"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func task(id int64, parent *int64, start, finish string, done float64, summary bool) contract.Task {
	return contract.Task{ID: id, ParentID: parent, Name: ptr("t"), Start: ptr(start + "T08:00:00"), Finish: ptr(finish + "T17:00:00"),
		PercentComplete: done, IsSummary: summary}
}

func window(t *testing.T, from, weeks string) Window {
	t.Helper()
	w, err := Parse(from, weeks)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestATaskIsInTheWindowWhenItOverlapsIt(t *testing.T) {
	w := window(t, "2026-04-06", "3")

	tests := []struct {
		name   string
		task   contract.Task
		inside bool
	}{
		{"starts inside", task(1, nil, "2026-04-20", "2026-05-15", 0, false), true},
		{"already under way", task(1, nil, "2026-03-20", "2026-04-08", 40, false), true},
		{"spans the whole window", task(1, nil, "2026-03-01", "2026-06-01", 10, false), true},
		{"starts on the last day", task(1, nil, "2026-04-26", "2026-05-02", 0, false), true},
		{"starts the day after", task(1, nil, "2026-04-27", "2026-05-02", 0, false), false},
		{"finished the day before", task(1, nil, "2026-03-20", "2026-04-05", 0, false), false},
		{"finishes on the first day", task(1, nil, "2026-03-20", "2026-04-06", 0, false), true},
		{"complete", task(1, nil, "2026-04-07", "2026-04-09", 100, false), false},
		{"a summary on its own", task(1, nil, "2026-04-07", "2026-04-09", 0, true), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := w.Holds(tt.task); got != tt.inside {
				t.Errorf("Holds = %v, want %v", got, tt.inside)
			}
		})
	}
}

func TestTheWindowKeepsThePhasesAboveItsTasks(t *testing.T) {
	root, phase, other := int64(0), int64(1), int64(4)
	c := &contract.Contract{Tasks: []contract.Task{
		task(0, nil, "2026-01-01", "2026-12-01", 0, true),
		task(1, &root, "2026-03-01", "2026-05-01", 0, true),
		task(2, &phase, "2026-04-07", "2026-04-09", 0, false),
		task(3, &phase, "2026-01-07", "2026-01-09", 0, false),
		task(4, &root, "2026-08-01", "2026-09-01", 0, true),
		task(5, &other, "2026-08-07", "2026-08-09", 0, false),
	}}

	kept := window(t, "2026-04-06", "2").Select(c)

	for id, want := range map[int64]bool{0: true, 1: true, 2: true, 3: false, 4: false, 5: false} {
		if kept[id] != want {
			t.Errorf("task %d kept = %v, want %v", id, kept[id], want)
		}
	}
}

func TestOnlyASaneWindowIsAccepted(t *testing.T) {
	for _, tt := range [][2]string{{"", "3"}, {"2026-04-06", ""}, {"06/04/2026", "3"}, {"2026-04-06", "0"}, {"2026-04-06", "13"}, {"2026-04-06", "three"}} {
		if _, err := Parse(tt[0], tt[1]); err == nil {
			t.Errorf("Parse(%q, %q) accepted", tt[0], tt[1])
		}
	}

	w := window(t, "2026-04-06", "3")
	if got := w.LastDay().Format("2006-01-02"); got != "2026-04-26" {
		t.Errorf("last day = %s, want 2026-04-26", got)
	}
}
