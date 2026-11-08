package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

type claudeInspectionCase struct {
	name        string
	sync        bool
	model       string
	nativeModel string
	settings    string
	ready       bool
	override    bool
	// issue and repair are what inspection reports for settings that
	// synchronization refuses; without them it reports drift to synchronize.
	issue  string
	repair string
}

func TestClaudeInspectionRequiresTheSynchronizedProjection(t *testing.T) {
	tests := []claudeInspectionCase{
		{name: "missing", model: "claude-model"},
		{name: "converged", sync: true, model: "claude-model", ready: true},
		{name: "native model preference", sync: true, model: "claude-model", nativeModel: "opus[1m]", ready: true, override: true},
		{name: "selected model changed", sync: true, model: "another-model"},
		{name: "malformed", sync: true, model: "claude-model", settings: "{", issue: "parse Claude settings", repair: "Correct the JSON in Claude Code's settings file, then run the same command again."},
		{name: "externally changed", sync: true, model: "claude-model", settings: `{"model":"external-model"}`, issue: "changed outside AIGW", repair: adapter.UndoOutsideEdit("Claude Code")},
		{name: "foreign credential before AIGW owns the file", model: "claude-model", settings: `{"apiKeyHelper":"foreign-helper"}`, issue: "credential ownership conflict", repair: "Remove the plaintext credential or foreign apiKeyHelper from Claude Code's settings, then run the same command again."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertClaudeInspectionCase(t, test)
		})
	}
}

func assertClaudeInspectionCase(t *testing.T, test claudeInspectionCase) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	if goruntime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(testAdmission())
	cfg.SetClientActivation(ID, true, executable, nil)
	settings := filepath.Join(root, ".claude", "settings.json")
	deps := adapter.Dependencies{Host: hostAt(root), AIGWExecutable: filepath.Join(root, "aigw")}
	runtime := domain.Runtime{ProviderID: "gateway", ModelID: "claude", UpstreamModel: "claude-model", Endpoint: "https://gateway.test"}
	if test.sync {
		if _, err := ReconcileSettings(settings, false, runtime, deps.AIGWExecutable, runtime.UpstreamModel); err != nil {
			t.Fatal(err)
		}
	}
	runtime.UpstreamModel = test.model
	if test.nativeModel != "" {
		data, err := os.ReadFile(settings)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		document["model"], _ = json.Marshal(test.nativeModel)
		data, _ = json.MarshalIndent(document, "", "  ")
		if err := os.WriteFile(settings, append(data, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if test.settings != "" {
		if err := os.MkdirAll(filepath.Dir(settings), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(settings, []byte(test.settings), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]platform.FileSnapshot{}
	for _, path := range []string{settings, settings + ".aigw-state.json"} {
		data, err := platform.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	status := (claudeAdapter{}).Inspect(context.Background(), deps, cfg, runtime)
	if status.Ready != test.ready || status.NativeModelOverride != test.override {
		t.Errorf("inspection = %+v, want ready=%t override=%t", status, test.ready, test.override)
	}
	issue, repair := "not synchronized", "aigw sync"
	if test.repair != "" {
		issue, repair = test.issue, test.repair
	}
	if !test.ready && (!strings.Contains(status.Issue, issue) || status.RepairAction != repair) {
		t.Errorf("inspection diagnosis = %+v, want an issue with %q and the repair %q", status, issue, repair)
	}
	if strings.Contains(status.Issue, root) {
		t.Errorf("Claude inspection exposed a private path: %+v", status)
	}
	for path, before := range files {
		after, err := platform.CaptureFileSnapshot(path)
		if err != nil || !after.Equal(before) {
			t.Errorf("inspection changed %s: %v", path, err)
		}
	}
}
