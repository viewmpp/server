package examples

import "testing"

func TestEveryExampleCountsItsOwnPlan(t *testing.T) {
	for _, e := range All() {
		t.Run(e.Name, func(t *testing.T) {
			stats := e.Stats()

			if stats.Tasks == 0 || stats.Relations == 0 || stats.Resources == 0 {
				t.Fatalf("counts came out empty: %+v", stats)
			}

			if stats.Critical == 0 || stats.Milestones == 0 || stats.Summaries == 0 {
				t.Errorf("a sample with no critical tasks, milestones or summaries exercises nothing: %+v", stats)
			}

			if stats.Depth < 2 {
				t.Errorf("outline depth is %d: a flat plan proves nothing about hierarchy", stats.Depth)
			}
		})
	}
}

func TestExamplesCarryTheirOwnCopy(t *testing.T) {
	seen := map[string]string{}

	for _, e := range All() {
		if e.Lead == "" {
			t.Errorf("%s has no lead", e.Name)
		}

		if len(e.Story) < 2 {
			t.Errorf("%s has %d story paragraphs, want at least 2", e.Name, len(e.Story))
		}

		if e.Format().Path == "" {
			t.Errorf("%s points at no format page", e.Name)
		}

		for _, paragraph := range append([]string{e.Lead}, e.Story...) {
			if other, repeated := seen[paragraph]; repeated {
				t.Errorf("%s repeats a paragraph from %s", e.Name, other)
			}
			seen[paragraph] = e.Name
		}
	}
}
