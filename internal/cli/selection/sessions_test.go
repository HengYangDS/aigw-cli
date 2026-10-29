package selection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/discovery"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secrets"
)

func TestUseTellsHermesUsersThatExistingSessionsKeepTheirRoute(t *testing.T) {
	const note = "Keep the route they started with; new sessions use this selection"
	runtime, cfg, out := configuredRuntime(t)
	store := secrets.NewMemoryStore()
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
		Surfaces:    []discovery.Surface{{ID: "hermes-home-default", Product: "Hermes", Authority: "aigw", ConfigPath: target}},
	}}
	for _, id := range []string{"first", "second"} {
		cfg.Routes[id] = domain.Route{
			Label: id, Account: "gateway", Model: id + "-model",
			Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
		}
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	use := func(args ...string) string {
		t.Helper()
		out.Reset()
		command := NewUseCommand(runtime)
		command.SilenceErrors = true
		command.SilenceUsage = true
		command.SetArgs(append([]string{"--for", hermes.ID}, args...))
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
		{"first selection", []string{"first"}, true},
		{"unchanged selection", []string{"first"}, false},
		{"preview of another Route", []string{"--dry-run", "second"}, true},
		{"preview of the selected Route", []string{"--dry-run", "first"}, false},
		{"another Route as JSON", []string{"--json", "second"}, false},
	} {
		output := use(step.args...)
		if strings.Contains(output, "Existing sessions") != step.want || strings.Contains(output, note) != step.want {
			t.Fatalf("%s: existing-session statement present=%t, want %t:\n%s", step.name, strings.Contains(output, note), step.want, output)
		}
	}
}
