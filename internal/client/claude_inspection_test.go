package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/transaction"
)

type claudeInspectionCase struct {
	name        string
	sync        bool
	model       string
	nativeModel string
	settings    string
	ready       bool
	override    bool
}

func TestClaudeInspectionRequiresTheSynchronizedProjection(t *testing.T) {
	tests := []claudeInspectionCase{
		{name: "missing", model: "claude-model"},
		{name: "converged", sync: true, model: "claude-model", ready: true},
		{name: "native model preference", sync: true, model: "claude-model", nativeModel: "opus[1m]", ready: true, override: true},
		{name: "selected model changed", sync: true, model: "another-model"},
		{name: "malformed", sync: true, model: "claude-model", settings: "{"},
		{name: "externally changed", sync: true, model: "claude-model", settings: `{"model":"external-model"}`},
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
	cfg := configuration.NewConfig()
	cfg.SetClientActivation(configuration.ClientClaude, true, executable, nil)
	deps := Dependencies{ClaudeSettingsPath: filepath.Join(root, "settings.json"), AIGWExecutable: filepath.Join(root, "aigw")}
	runtime := configuration.Runtime{AccountID: "gateway", RouteID: "claude", Model: "claude-model", Endpoint: "https://gateway.test"}
	if test.sync {
		if _, err := claude.ReconcileSettings(deps.ClaudeSettingsPath, false, runtime, deps.AIGWExecutable, runtime.Model); err != nil {
			t.Fatal(err)
		}
	}
	runtime.Model = test.model
	if test.nativeModel != "" {
		data, err := os.ReadFile(deps.ClaudeSettingsPath)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		document["model"], _ = json.Marshal(test.nativeModel)
		data, _ = json.MarshalIndent(document, "", "  ")
		if err := os.WriteFile(deps.ClaudeSettingsPath, append(data, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if test.settings != "" {
		if err := os.WriteFile(deps.ClaudeSettingsPath, []byte(test.settings), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]transaction.FileSnapshot{}
	for _, path := range []string{deps.ClaudeSettingsPath, deps.ClaudeSettingsPath + ".aigw-state.json"} {
		data, err := transaction.CaptureFileSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	status := (claudeAdapter{}).Inspect(context.Background(), deps, cfg, runtime)
	if status.Ready != test.ready || status.NativeModelOverride != test.override {
		t.Errorf("inspection = %+v, want ready=%t override=%t", status, test.ready, test.override)
	}
	if !test.ready && (!strings.Contains(status.Issue, "not synchronized") || status.RepairAction != "aigw sync") {
		t.Errorf("missing synchronization diagnosis: %+v", status)
	}
	for path, before := range files {
		after, err := transaction.CaptureFileSnapshot(path)
		if err != nil || !after.Equal(before) {
			t.Errorf("inspection changed %s: %v", path, err)
		}
	}
}
