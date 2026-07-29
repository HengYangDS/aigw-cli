package cli

import (
	"path/filepath"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestMutationCommandLocksEveryConfigurationWriter(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "top-level add", args: []string{"add", "account"}, want: true},
		{name: "account edit", args: []string{"account", "edit", "account"}, want: true},
		{name: "account connect", args: []string{"account", "connect", "account"}, want: true},
		{name: "account rename", args: []string{"account", "rename", "old", "new"}, want: true},
		{name: "account rename dry-run", args: []string{"account", "rename", "old", "new", "--dry-run"}, want: false},
		{name: "account rename dry-run equals", args: []string{"account", "rename", "old", "new", "--dry-run=true"}, want: false},
		{name: "account rename dry-run false", args: []string{"account", "rename", "old", "new", "--dry-run=false"}, want: true},
		{name: "account rename finalize", args: []string{"account", "rename", "old", "new", "--finalize"}, want: true},
		{name: "account rename finalize dry-run", args: []string{"account", "rename", "old", "new", "--finalize", "--dry-run"}, want: false},
		{name: "profile add", args: []string{"profile", "add", "profile"}, want: true},
		{name: "profile edit", args: []string{"profile", "edit", "profile"}, want: true},
		{name: "profile rename", args: []string{"profile", "rename", "old", "new"}, want: true},
		{name: "profile rename dry-run", args: []string{"profile", "rename", "old", "new", "--dry-run"}, want: false},
		{name: "profile rename dry-run equals", args: []string{"profile", "rename", "old", "new", "--dry-run=true"}, want: false},
		{name: "profile rename dry-run false", args: []string{"profile", "rename", "old", "new", "--dry-run=false"}, want: true},
		{name: "profile remove", args: []string{"profile", "remove", "profile"}, want: true},
		{name: "profile list", args: []string{"profile", "list"}, want: false},
		{name: "profile show", args: []string{"profile", "show", "profile"}, want: false},
		{name: "account list", args: []string{"account", "list"}, want: false},
		{name: "removed config upgrade", args: []string{"config", "upgrade"}, want: false},
		{name: "repair apply", args: []string{"repair"}, want: true},
		{name: "repair dry-run", args: []string{"repair", "--dry-run"}, want: false},
		{name: "repair dry-run equals", args: []string{"repair", "--dry-run=true"}, want: false},
		{name: "update", args: []string{"update"}, want: true},
		{name: "bare account", args: []string{"account"}, want: false},
		{name: "bare profile", args: []string{"profile"}, want: false},
		{name: "bare route", args: []string{"route"}, want: false},
		{name: "route reset", args: []string{"route", "reset"}, want: true},
		{name: "route unknown verb", args: []string{"route", "unknown"}, want: false},
		{name: "bare adapter", args: []string{"adapter"}, want: false},
		{name: "adapter enable", args: []string{"adapter", "enable", "codex"}, want: true},
		{name: "adapter auth", args: []string{"adapter", "auth", "codex"}, want: true},
		{name: "adapter disable", args: []string{"adapter", "disable", "codex"}, want: true},
		{name: "adapter unknown verb", args: []string{"adapter", "discover"}, want: false},
		{name: "bare config", args: []string{"config"}, want: false},
		{name: "config import", args: []string{"config", "import", "path"}, want: true},
		{name: "config unknown verb", args: []string{"config", "export"}, want: false},
		{name: "unknown top-level", args: []string{"status"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mutationCommand(&App{}, tt.args); got != tt.want {
				t.Fatalf("mutationCommand(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestMutationCommandWithNoArgsChecksInteractiveOnboarding(t *testing.T) {
	emptyStore := App{Config: config.NewStore(filepath.Join(t.TempDir(), "missing.toml")), Interactive: true}
	if !mutationCommand(&emptyStore, nil) {
		t.Fatal("an interactive terminal with no profiles should trigger the onboarding wizard lock")
	}

	nonInteractive := App{Config: config.NewStore(filepath.Join(t.TempDir(), "missing.toml")), Interactive: false}
	if mutationCommand(&nonInteractive, nil) {
		t.Fatal("a non-interactive session with no profiles must not take a mutation lock")
	}

	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	cfg := domain.NewConfig()
	cfg.Accounts["dmx"] = domain.Account{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "dmx", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	cfg.Routes.Default = "gpt"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	populated := App{Config: store, Interactive: true}
	if mutationCommand(&populated, nil) {
		t.Fatal("an already-configured store must not trigger the onboarding wizard lock")
	}
}

func TestCodexProjectionChangeIgnoresProfilePurpose(t *testing.T) {
	before := domain.NewConfig()
	before.Accounts["gateway"] = domain.Account{Label: "Gateway", Endpoints: domain.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
	before.Profiles["gpt"] = domain.Profile{Label: "GPT", Account: "gateway", Client: domain.ClientCodex, Models: domain.Models{domain.ClientCodex: "gpt-test"}}
	before.Routes.Default = "gpt"
	before.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Targets: []string{"/tmp/codex.toml"}}

	after := cloneConfig(before)
	profile := after.Profiles["gpt"]
	profile.Purpose = "Code and engineering"
	after.Profiles["gpt"] = profile

	if codexProjectionChanged(before, after) {
		t.Fatal("display-only profile purpose must not rewrite the Codex projection")
	}
}
