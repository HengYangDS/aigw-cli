package sync

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/terminal"
)

// A client whose configured executable is missing stops alone: sync writes the
// other clients' files, keeps every file of the stopped client as it was, and
// fails with the missing executable and the one fix that names another, never
// a sync. Its preview fails the same way and writes nothing.
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
			cfg.SetClientActivation(claude.ID, true, fakeProgram("/opt/claude"), nil)
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
			runtime := invocation.Context{Clients: clients.Registry(), Config: configStore, Executable: source, Secrets: tokens, Out: &bytes.Buffer{}, Discovery: staticDiscovery{}, Host: hostAt(root)}
			cmd := NewCommand(runtime)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			args := []string{"--json"}
			if dryRun {
				args = append(args, "--dry-run")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()

			problem := terminal.ProblemFor(err)
			if _, stopped := errors.AsType[*adapter.MissingProgramError](err); !stopped || problem.Title != "Codex executable is missing" || problem.Fix != "aigw client edit codex --executable <path>" {
				t.Fatalf("sync = %v (%+v), want Codex stopped for its missing executable", err, problem)
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
