package store

import (
	"aigw-cli/internal/domain"
	"testing"
)

// The shipped file declares a context window only below the default, and the
// models of one family agree: each declares the family's window, or none of
// them declares one. Which families and which numbers is the file's own
// business and the qualification evidence's.
func TestTeamManifestDeclaresWindowsOnlyBelowTheDefaultAndPerFamily(t *testing.T) {
	_, manifest := loadTeamManifest(t)
	declared := map[string]int{}
	seen := map[string]domain.ModelRef{}
	for _, ref := range manifest.ModelRefs() {
		model := manifest.Models[ref]
		if model.ContextWindow != 0 && model.ContextWindow >= domain.DefaultContextWindow {
			t.Errorf("%s declares a context window of %d, which is not below the default of %d", ref, model.ContextWindow, domain.DefaultContextWindow)
		}
		family := model.FamilyID()
		first, known := seen[family]
		if !known {
			seen[family], declared[family] = ref, model.ContextWindow
			continue
		}
		if model.ContextWindow != declared[family] {
			t.Errorf("family %s: %s declares a context window of %d and %s one of %d (0 is none)", family, first, declared[family], ref, model.ContextWindow)
		}
	}
}
