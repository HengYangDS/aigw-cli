package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/processtest"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// A client whose configured executable is missing stops alone: sync writes the
// other clients' files, keeps every file of the stopped client as it was, and
// fails with the missing executable and the one fix that names another, never
// a sync. Its preview writes nothing, lists the other clients' files and the
// stopped client with its problem and fix, and fails too.
func TestSyncStopsAClientWhoseExecutableIsMissing(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run("dry run "+strconv.FormatBool(dryRun), func(t *testing.T) {
			configStore, cfg := configuredRepairStore(t)
			root := t.TempDir()
			cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test", Responses: "https://one.test/v1"}}
			cfg.Models[domain.NewModelRef("one", "gpt-test")] = domain.Model{
				Label: "GPT", Provider: "one", Family: "gpt-test", ID: "gpt-test",
				Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}},
			}
			cfg.SetSelectedModel(codex.ID, domain.NewModelRef("one", "gpt-test"), "")
			target := filepath.Join(root, "codex", "config.toml")
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(codex.ID, true, filepath.Join(root, "gone", "codex"), []string{target})
			cfg.SetClientActivation(claude.ID, true, processtest.Program(t, "/opt/claude"), nil)
			if err := configStore.Save(cfg); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(root, "aigw")
			if err := os.WriteFile(source, []byte("AIGW fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			tokens := secret.NewMemoryStore()
			if err := tokens.Set("one", "synthetic-fixture"); err != nil {
				t.Fatal(err)
			}
			out := &bytes.Buffer{}
			runtime := invocation.Context{Clients: clients.Registry(), Stops: &synchronization.Stops{}, Config: configStore, Executable: source, Secrets: tokens, Out: out, Discovery: staticDiscovery{}, Host: hostAt(root)}
			cmd := NewCommand(runtime)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			args := []string{"--json"}
			if dryRun {
				args = append(args, "--dry-run")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()

			if _, stopped := errors.AsType[*adapter.StopError](err); !stopped {
				t.Fatalf("sync = %v, want Codex stopped for its missing executable", err)
			}
			if dryRun {
				requirePreviewStop(t, out.Bytes())
			} else if problem := terminal.ProblemFor(err); problem.Title != "Codex executable is missing" || problem.Fix != "aigw client edit codex --executable <path>" {
				t.Fatalf("sync = %+v, want the missing executable and its fix", problem)
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != "model_provider = \"native\"\n" {
				t.Fatalf("sync changed Codex's file: %q, %v", data, err)
			}
			entries, err := os.ReadDir(filepath.Dir(target))
			if err != nil || len(entries) != 1 {
				t.Fatalf("sync wrote beside Codex's file: %v, %v", entries, err)
			}
			_, settingsErr := os.Stat(filepath.Join(root, ".claude", "settings.json"))
			if written := settingsErr == nil; written == dryRun {
				t.Fatalf("Claude Code's settings written = %t in a dry run %t: %v", written, dryRun, settingsErr)
			}
		})
	}
}

// requirePreviewStop requires a preview document that plans Claude Code's file
// alone, fails, and names Codex stopped with its problem and fix.
func requirePreviewStop(t *testing.T, data []byte) {
	t.Helper()
	var document struct {
		OK      bool                       `json:"ok"`
		Stopped []invocation.StoppedClient `json:"stopped_clients"`
		Files   []invocation.ClientFile    `json:"client_files"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	want := invocation.StoppedClient{Client: codex.ID, Problem: "Codex executable is missing", NextDetail: "aigw client edit codex --executable <path>"}
	if document.OK || len(document.Stopped) != 1 || document.Stopped[0] != want || len(document.Files) != 1 || document.Files[0].Client != claude.ID {
		t.Fatalf("preview = %s, want Claude Code's file and Codex stopped", data)
	}
}
