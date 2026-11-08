package claudedesktop

import (
	"os"
	"strings"
	"testing"

	"aigw-cli/internal/terminal"
)

// userDesktop gives Claude Desktop the user's own deployment mode and profile
// before AIGW projects, and returns the paths and the request.
func userDesktop(t *testing.T) (Paths, Desired) {
	t.Helper()
	paths, desired := desktopFixture(t)
	if _, err := Prepare(paths, nil); err != nil {
		t.Fatal(err)
	}
	withdrawal, err := Prepare(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withdrawal.Apply(); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, paths.StandardConfig, map[string]any{"deploymentMode": "1p", "preferences": map[string]any{"keep": true}})
	writeJSON(t, paths.ThirdPartyConfig, map[string]any{"deploymentMode": "1p"})
	writeJSON(t, paths.Metadata, map[string]any{"appliedId": "mine", "entries": []any{map[string]any{"id": "mine", "name": "Mine"}}})
	applyProjection(t, paths, desired)
	return paths, desired
}

// With the profile file gone and the record intact, a withdrawal restores the
// user's deployment mode and selected profile from the record, keeps the
// user's own entries and removes the record. A projection refuses, naming
// withdrawal as the step that keeps the user's settings.
func TestAWithdrawalRestoresTheRecordedSettingsWithoutTheProfile(t *testing.T) {
	paths, desired := userDesktop(t)
	if err := os.Remove(paths.Profile); err != nil {
		t.Fatal(err)
	}
	_, err := Prepare(paths, &desired)
	if problem := terminal.ProblemFor(err); !strings.Contains(problem.Fix, "aigw client disable claude-desktop") {
		t.Fatalf("a projection without the profile = %+v", problem)
	}
	withdrawal, err := Prepare(paths, nil)
	if err != nil || withdrawal.Action != ActionRestore {
		t.Fatalf("the withdrawal = %#v, %v", withdrawal, err)
	}
	if _, err := withdrawal.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, paths.StandardConfig); got["deploymentMode"] != "1p" || got["preferences"] == nil {
		t.Fatalf("the standard configuration = %#v", got)
	}
	if got := readJSON(t, paths.ThirdPartyConfig); got["deploymentMode"] != "1p" {
		t.Fatalf("the third-party configuration = %#v", got)
	}
	metadata := readJSON(t, paths.Metadata)
	if entries, _ := metadata["entries"].([]any); metadata["appliedId"] != "mine" || len(entries) != 1 {
		t.Fatalf("the file list = %#v", metadata)
	}
	if _, err := os.Stat(paths.State); !os.IsNotExist(err) {
		t.Fatalf("the record after the withdrawal: %v", err)
	}
}

// A withdrawal without the profile still refuses when a setting that AIGW
// recorded was changed outside it, naming the setting.
func TestAWithdrawalWithoutTheProfileRefusesAChangedSetting(t *testing.T) {
	paths, _ := userDesktop(t)
	if err := os.Remove(paths.Profile); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, paths.StandardConfig, map[string]any{"deploymentMode": "1p"})
	if _, err := Prepare(paths, nil); err == nil || !strings.Contains(err.Error(), "deployment mode") {
		t.Fatalf("the withdrawal = %v", err)
	}
}
