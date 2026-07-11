package config_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestLoadThenSaveRemovesDuplicatedProfileEndpointResidue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	legacy := `version = 1

[accounts.gateway]
label = "Gateway"

[accounts.gateway.endpoints]
openai_responses = "https://gateway.test/v1"

[profiles.gpt]
label = "GPT"
account = "gateway"
client = "codex"

[profiles.gpt.models]
codex = "gpt-test"

[profiles.gpt.endpoints]
openai_responses = "https://duplicate.test/v1"

[profiles.gpt.account_probe]
kind = "dmxapi"
base_url = "https://duplicate.test"

[routes]
default = "gpt"
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(path)
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, residue := range []string{"[profiles.gpt.endpoints]", "[profiles.gpt.account_probe]", "https://duplicate.test"} {
		if strings.Contains(text, residue) {
			t.Fatalf("canonical save retained legacy Profile residue %q:\n%s", residue, text)
		}
	}
	if !strings.Contains(text, "[accounts.gateway.endpoints]") || !strings.Contains(text, "https://gateway.test/v1") {
		t.Fatalf("canonical save lost Account endpoint:\n%s", text)
	}
}

func TestLockSerializesMutations(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.toml"))
	unlock, err := store.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err = store.Lock(ctx)
	if err == nil || !strings.Contains(err.Error(), "context deadline") {
		t.Fatalf("second lock error = %v", err)
	}
}

func TestSaveLoadRoundTripAndSecurePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	store := config.NewStore(path)
	want := domain.Config{
		Version:  1,
		Accounts: map[string]domain.Account{"dmx": {Label: "DMXAPI", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}},
		Profiles: map[string]domain.Profile{"dmx": {Label: "DMXAPI", Account: "dmx"}},
		Routes:   domain.Routes{Default: "dmx", Overrides: map[string]string{}},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts["dmx"].Endpoints.Anthropic != "https://example.test" || got.Routes.Default != "dmx" {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestSaveRefusesInvalidConfigWithoutReplacingExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(path)
	if err := store.Save(domain.Config{Version: 1}); err == nil {
		t.Fatal("expected validation error")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "sentinel" {
		t.Fatalf("existing config was replaced: %q", got)
	}
}

func TestSaveKeepsOneSecretFreePreviousVersionBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	first := domain.Config{
		Version:  1,
		Accounts: map[string]domain.Account{"one": {Label: "One", Endpoints: domain.Endpoints{Anthropic: "https://one.test"}}},
		Profiles: map[string]domain.Profile{"one": {Label: "One", Account: "one"}},
		Routes:   domain.Routes{Default: "one", Overrides: map[string]string{}},
	}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Accounts = map[string]domain.Account{"two": {Label: "Two", Endpoints: domain.Endpoints{Anthropic: "https://two.test"}}}
	second.Profiles = map[string]domain.Profile{"two": {Label: "Two", Account: "two"}}
	second.Routes.Default = "two"
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backup), `[profiles.one]`) || strings.Contains(strings.ToLower(string(backup)), "token") {
		t.Fatalf("backup = %s", backup)
	}
}

func TestVerifiedCheckpointRoundTripIsSecretFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := config.NewStore(path)
	cfg := domain.Config{
		Version:  1,
		Accounts: map[string]domain.Account{"dmx": {Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}},
		Profiles: map[string]domain.Profile{"claude": {Label: "Claude", Account: "dmx", Models: domain.Models{Claude: "claude-test"}}},
		Routes:   domain.Routes{Default: "claude", Overrides: map[string]string{}},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(cfg, []string{"claude", "codex"}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Config.Routes.Default != "claude" || len(checkpoint.Clients) != 2 || checkpoint.VerifiedAt.IsZero() {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
	data, err := os.ReadFile(path + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "token") {
		t.Fatalf("checkpoint contains token-like content: %s", data)
	}
}
