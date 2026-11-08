package claude

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/terminal"
)

// Settings that still hold AIGW's credential helper without AIGW's record
// cannot be withdrawn: the record alone holds what AIGW replaced. The
// withdrawal refuses, says what stays, names the record to restore, and
// leaves the file as it was. Settings that AIGW never touched stay restored.
func TestAWithdrawalWithoutTheRecordRefusesAndSaysWhatStays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark","model":"user-model"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime := domain.Runtime{ModelID: "team", ProviderID: "gateway", Endpoint: "https://gateway.test", UpstreamModel: "team-model"}
	if _, err := ReconcileSettings(path, false, runtime, testExecutable(), runtime.UpstreamModel); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path + adapter.StateSuffix); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReconcileSettings(path, true, domain.Runtime{}, "", runtime.UpstreamModel)
	problem := terminal.ProblemFor(err)
	if !strings.Contains(problem.Title, "apiKeyHelper stays") || !strings.Contains(problem.Fix, "Restore settings.json.aigw-state.json") {
		t.Fatalf("the refusal = %+v", problem)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, before) {
		t.Fatal("the refusal changed the settings")
	}
	untouched := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(untouched, []byte(`{"apiKeyHelper":"my-own-helper"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if receipt, err := ReconcileSettings(untouched, true, domain.Runtime{}, "", ""); err != nil || receipt.Action != SettingsActionAlreadyRestored {
		t.Fatalf("settings that AIGW never touched = %#v, %v", receipt, err)
	}
}
