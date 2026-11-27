package sync

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/app/synchronization"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/processtest"
	"aigw-cli/internal/secret"
)

// A client whose configured executable is missing stops alone: sync writes the
// other clients' files, keeps every file of the stopped client as it was, and
// records the stop, with the missing executable and the one fix that names
// another, never a sync, which the invocation reports after sync's own report.
// Its preview writes nothing, plans the other clients' files and records the
// stop too.
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

			stops := runtime.Stops.Stopped()
			if err != nil || len(stops) != 1 || stops[0].Client() != codex.ID || stops[0].Error() != "Codex program is missing" || stops[0].NextAction() != "aigw client edit codex --program <path> or --extensions <dir>" {
				t.Fatalf("sync = %v with the stops %v, want sync's report and Codex's stop recorded", err, stops)
			}
			if written, withdrawn, preview := runtime.Stops.Written(); len(written) != 1 || written[0] != claude.ID || len(withdrawn) != 0 || preview != dryRun {
				t.Fatalf("the record holds written %v, withdrawn %v, preview %t", written, withdrawn, preview)
			}
			if dryRun {
				requirePreviewPlan(t, out.Bytes())
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

// requirePreviewPlan requires a preview document that plans Claude Code's file
// alone.
func requirePreviewPlan(t *testing.T, data []byte) {
	t.Helper()
	var document struct {
		Files []invocation.ClientFile `json:"client_files"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Files) != 1 || document.Files[0].Client != claude.ID {
		t.Fatalf("preview = %s, want Claude Code's file alone", data)
	}
}
