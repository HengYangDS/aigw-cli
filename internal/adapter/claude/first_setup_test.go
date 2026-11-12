package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
)

// The first setup after an upgrade has no earlier selection, so the model of
// the new selection stands in for the one that the earlier release projected.
func TestFirstSetupAcceptsAModelPreferenceOfAnEarlierRelease(t *testing.T) {
	runtime := domain.Runtime{Model: domain.NewModelRef("gateway", "claude-team"), ProviderID: "gateway", Endpoint: "https://gateway.test", UpstreamModel: "claude-team"}
	for _, test := range []struct {
		name      string
		edit      func(settingsDocument)
		wantModel string
	}{
		{"model removed", func(document settingsDocument) { delete(document, "model") }, ""},
		{"model replaced", func(document settingsDocument) { document["model"] = json.RawMessage(`"claude-other"`) }, `"claude-other"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.UpstreamModel); err != nil {
				t.Fatal(err)
			}
			document := readSettingsFile(t, path)
			test.edit(document)
			data, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err != nil {
				t.Fatalf("the first setup refused the user's model preference: %v", err)
			}
			got, present := readSettingsFile(t, path)["model"]
			if (test.wantModel == "") == (present) || (present && string(got) != test.wantModel) {
				t.Fatalf("model = %s (present %t), want %q kept", got, present, test.wantModel)
			}
			// Any other change of a managed value stays a conflict.
			document = readSettingsFile(t, path)
			data, _ = json.Marshal(document)
			data = []byte(strings.Replace(string(data), "https://gateway.test", "https://foreign.test", 1))
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileSettings(path, false, runtime, testExecutable(), ""); err == nil || !strings.Contains(err.Error(), "managed Claude settings changed") {
				t.Fatalf("a changed endpoint on a first setup: %v", err)
			}
		})
	}
}
