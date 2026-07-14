package cli

import (
	"testing"

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
		{name: "profile add", args: []string{"profile", "add", "profile"}, want: true},
		{name: "profile edit", args: []string{"profile", "edit", "profile"}, want: true},
		{name: "profile rename", args: []string{"profile", "rename", "old", "new"}, want: true},
		{name: "profile remove", args: []string{"profile", "remove", "profile"}, want: true},
		{name: "profile list", args: []string{"profile", "list"}, want: false},
		{name: "profile show", args: []string{"profile", "show", "profile"}, want: false},
		{name: "account list", args: []string{"account", "list"}, want: false},
		{name: "removed config upgrade", args: []string{"config", "upgrade"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mutationCommand(&App{}, tt.args); got != tt.want {
				t.Fatalf("mutationCommand(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
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
