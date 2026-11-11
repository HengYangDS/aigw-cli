package sync

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/store"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

type staticDiscovery struct{ result discovery.Result }

// hostAt observes home as the user's home directory, so Claude Code's settings
// file is home/.claude/settings.json.
func hostAt(home string) discovery.Host {
	return discovery.Host{GOOS: goruntime.GOOS, Home: home, Env: map[string]string{"HOME": home, "USERPROFILE": home}}
}

func (candidate staticDiscovery) Discover(...string) discovery.Result { return candidate.result }

func configuredRepairStore(t *testing.T) (store.Store, domain.Config) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["one"] = domain.Provider{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Models[domain.NewModelRef("one", "claude-test")] = domain.Model{
		Label: "One", Provider: "one", Family: "claude-test", ID: "claude-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-test"), "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return configStore, cfg
}
