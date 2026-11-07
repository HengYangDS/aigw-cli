package client

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/claudedesktop"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli/invocation"
)

// executableFixture writes a runnable file that stands for a client's
// executable.
func executableFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// listed runs `aigw client list --json` and returns its clients.
func listed(t *testing.T) (int, []clientItem) {
	t.Helper()
	var document struct {
		SchemaVersion int          `json:"schema_version"`
		Clients       []clientItem `json:"clients"`
	}
	runtime, out := listRuntime(t)
	if err := executeAdapter(t, runtime, "list", "--json"); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil {
		t.Fatalf("list document: %v\n%s", err, out)
	}
	return document.SchemaVersion, document.Clients
}

// The list reports every client in admission order with the executable that
// synchronization keeps, its enabled state and its model as provider/model.
// A disabled client keeps its selection, and the list reads no Token.
func TestListReportsEveryClientsExecutableStateAndModel(t *testing.T) {
	version, items := listed(t)
	if version != 2 {
		t.Fatalf("schema_version = %d", version)
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if !slices.Equal(ids, []string{claude.ID, claudedesktop.ID, codex.ID, hermes.ID}) {
		t.Fatalf("clients = %q", ids)
	}
	byID := map[string]clientItem{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if got := byID[claude.ID]; !got.Installed || !strings.HasSuffix(got.Executable, claude.ID) || !got.Enabled || got.Model != "gateway/claude-test" {
		t.Fatalf("Claude Code = %#v", got)
	}
	if got := byID[codex.ID]; !got.Installed || !strings.HasSuffix(got.Executable, "discovered-codex") || got.Enabled || got.Model != "gateway/codex-test" {
		t.Fatalf("a disabled Codex with a vanished executable = %#v", got)
	}
	if got := byID[hermes.ID]; got.Installed || got.Executable != "" || got.Enabled || got.Model != "" {
		t.Fatalf("an absent Hermes = %#v", got)
	}
}

func TestListShowsEachClientInOneRow(t *testing.T) {
	runtime, out := listRuntime(t)
	if err := executeAdapter(t, runtime, "list"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"✓ Claude ", "gateway/claude-test · Enabled · ",
		"· Codex ", "gateway/codex-test · Disabled · ",
		"· Hermes ", "No model selected · Disabled · Not installed",
		"aigw use <client> <provider/model>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("list lacks %q:\n%s", want, text)
		}
	}
}

func TestListMarksAnEnabledClientThatIsNotInstalled(t *testing.T) {
	runtime, out := listRuntime(t)
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claude.ID, true, filepath.Join(t.TempDir(), "gone"), nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.Discovery = adapterDiscovery{}
	if err := executeAdapter(t, runtime, "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "! Claude ") || !strings.Contains(out.String(), "gateway/claude-test · Enabled · Not installed") {
		t.Fatalf("an enabled client without an executable is not marked:\n%s", out)
	}
}

func TestListFailsWhenDiscoveryIsUnavailable(t *testing.T) {
	runtime, _ := listRuntime(t)
	runtime.Discovery = nil
	if err := executeAdapter(t, runtime, "list"); err == nil || !strings.Contains(err.Error(), "discovery is unavailable") {
		t.Fatalf("list error = %v", err)
	}
}

func TestInstalledExecutableKeepsAnAvailableConfiguredOne(t *testing.T) {
	configured, discovered := executableFixture(t, "configured"), executableFixture(t, "discovered")
	for _, test := range []struct {
		name, configured, discovered, want string
	}{
		{"configured", configured, discovered, configured},
		{"vanished", filepath.Join(t.TempDir(), "vanished"), discovered, discovered},
		{"discovered only", "", discovered, discovered},
		{"neither", "", "", ""},
	} {
		if got := installedExecutable(test.configured, test.discovered); got != test.want {
			t.Errorf("%s: installedExecutable() = %q, want %q", test.name, got, test.want)
		}
	}
}

// listRuntime is a configuration in which Claude Code is enabled with an
// executable that exists, Codex is disabled and its configured executable is
// gone while discovery finds another, and Hermes is absent. Its credential
// store fails on any use.
func listRuntime(t *testing.T) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	cfg := adapterConfig()
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, claude.ID), nil)
	cfg.SetClientActivation(codex.ID, false, filepath.Join(t.TempDir(), "vanished-codex"), nil)
	runtime, _, _, _ := adapterRuntime(t, cfg)
	runtime.Secrets = unavailableAdapterSecretStore{}
	runtime.Discovery = adapterDiscovery{result: discovery.Result{Executables: map[string]string{codex.ID: executableFixture(t, "discovered-codex")}}}
	out := &bytes.Buffer{}
	runtime.Out = out
	return runtime, out
}
