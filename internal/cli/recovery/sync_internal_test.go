package recovery

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	configuration "aigw-cli/internal/configuration"
)

func TestSyncPropagatesPlanningAndReconciliationFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "planning", args: []string{"--dry-run"}},
		{name: "reconciliation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			cfg := configuration.NewConfig()
			cfg.Accounts["one"] = configuration.Account{Label: "One", Endpoints: configuration.Endpoints{OpenAIResponses: "https://one.test/v1"}}
			cfg.Profiles["one"] = configuration.Profile{Label: "One", Account: "one", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt-test"}}
			cfg.Routes.Default = "one"
			cfg.Adapters[configuration.ClientCodex] = configuration.AdapterConfig{Enabled: true, Executable: "/portable/codex", Targets: []string{"/portable/config.toml"}}
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			out := &bytes.Buffer{}
			errOut := &bytes.Buffer{}
			command := NewSyncCommand(invocation.Context{Config: store, Out: out, RenderOut: out})
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetOut(out)
			command.SetErr(errOut)
			command.SetArgs(test.args)

			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), "discovery") {
				t.Fatalf("sync error = %v, want discovery failure", err)
			}
			if strings.Contains(out.String(), "completed") {
				t.Fatalf("sync reported completion after failure: %q", out.String())
			}
			combined := strings.ToLower(out.String() + errOut.String())
			for _, residue := range []string{"usage:", "warning", "traceback"} {
				if strings.Contains(combined, residue) {
					t.Fatalf("sync emitted %q after a handled failure: %q", residue, combined)
				}
			}
		})
	}
}
