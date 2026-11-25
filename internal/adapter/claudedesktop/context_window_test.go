package claudedesktop

import (
	"testing"

	"aigw-cli/internal/domain"
)

// Claude Desktop is given no context window, so a changed window of the model
// that it selects is not a changed projection of it: the edit neither plans
// nor inspects Claude Desktop's files.
func TestAChangedWindowIsNotAChangedClaudeDesktopProjection(t *testing.T) {
	fixture := newClaudeDesktopFixture(t)
	if _, err := fixture.clientAdapter.Apply(t.Context(), fixture.deps, domain.NewConfig(testAdmission()), fixture.cfg); err != nil {
		t.Fatal(err)
	}
	edited := fixture.cfg.Clone()
	ref := edited.Clients[ID].Model
	model := edited.Models[ref]
	model.ContextWindow = 400000
	edited.Models[ref] = model
	if fixture.clientAdapter.ProjectionChanged(fixture.cfg, edited) {
		t.Error("a changed context window of the selected model counts as a changed Claude Desktop projection")
	}
}
