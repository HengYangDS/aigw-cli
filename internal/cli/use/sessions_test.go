package use

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
)

func TestUseTellsHermesUsersThatExistingSessionsKeepTheirModel(t *testing.T) {
	const note = "Keep the model they started with; new sessions use this selection"
	runtime, cfg, out := configuredRuntime(t)
	store := secret.NewMemoryStore()
	if err := store.Set("gateway", "token"); err != nil {
		t.Fatal(err)
	}
	runtime.Secrets = store
	runtime.Executable = filepath.Join(t.TempDir(), "aigw")
	executable := filepath.Join(t.TempDir(), "hermes")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), ".hermes", "config.yaml")
	runtime.Discovery = staticDiscovery{result: discovery.Result{
		Executables: map[string]string{hermes.ID: executable},
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", ConfigPath: target}},
	}}
	for _, id := range []string{"first", "second"} {
		cfg.Models[id] = domain.Model{
			Label: id, Provider: "gateway", Family: id + "-model",
			Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}},
		}
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	use := func(args ...string) string {
		t.Helper()
		out.Reset()
		command := NewCommand(runtime)
		command.SilenceErrors = true
		command.SilenceUsage = true
		command.SetArgs(append([]string{hermes.ID}, args...))
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	for _, step := range []struct {
		name string
		args []string
		want bool
	}{
		{"first selection", []string{"gateway/first-model"}, true},
		{"unchanged selection", []string{"gateway/first-model"}, false},
		{"preview of another model", []string{"gateway/second-model", "--dry-run"}, true},
		{"preview of the selected model", []string{"gateway/first-model", "--dry-run"}, false},
		{"another model as JSON", []string{"gateway/second-model", "--json"}, false},
	} {
		output := use(step.args...)
		if strings.Contains(output, "Existing sessions") != step.want || strings.Contains(output, note) != step.want {
			t.Fatalf("%s: existing-session statement present=%t, want %t:\n%s", step.name, strings.Contains(output, note), step.want, output)
		}
	}
}
