package sync

import (
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/discovery"
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

func (candidate staticDiscovery) Discover() discovery.Result { return candidate.result }

func configuredRepairStore(t *testing.T) (store.Store, domain.Config) {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["one"] = domain.Account{Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}
	cfg.Routes["one"] = domain.Route{
		Label: "One", Account: "one", Model: "claude-test",
		Interfaces: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}},
	}
	cfg.SetSelectedRoute(claude.ID, "one", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return configStore, cfg
}
