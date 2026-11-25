package projection

import (
	"path/filepath"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"
)

// The budget of each GitHub job that runs the selection matrix is the measured
// work before the matrix and the matrix's bound, rounded up to whole ten
// minutes, so a slower measured run raises it. The rule is pinned here, not
// its result: the graph is evaluated with the real bound and a larger one.
func TestMatrixJobBudgetFollowsTheMatrixBound(t *testing.T) {
	var work struct {
		RunStep int `yaml:"runStepSeconds"`
		Setup   int `yaml:"setupSeconds"`
	}
	declaredValue(t, "selectionMatrixPrecedingWork", &work)
	if work.RunStep <= 0 || work.Setup <= 0 {
		t.Fatalf("the work before the selection matrix is not declared: %+v", work)
	}
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	previous := 0
	for _, bound := range []time.Duration{SelectionMatrixBound, SelectionMatrixBound + time.Hour} {
		output, err := exportCommand(root, "matrixJobMinutes", bound).Output()
		if err != nil {
			t.Fatalf("export the job budget for a %v bound: %v", bound, err)
		}
		var minutes int
		if err := yaml.Unmarshal(output, &minutes); err != nil {
			t.Fatal(err)
		}
		seconds := work.RunStep + work.Setup + int(bound/time.Second)
		if want := (seconds + 599) / 600 * 10; minutes != want || minutes <= previous {
			t.Fatalf("a %v bound gives a %d-minute job budget, want %d, above the %d of the smaller bound", bound, minutes, want, previous)
		}
		previous = minutes
	}
}
