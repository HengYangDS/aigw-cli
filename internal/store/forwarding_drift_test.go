package store

import (
	"aigw-cli/internal/domain"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const forwardingTestRoot = "http://127.0.0.1:8792"

// forwardTo sends Codex through the forwarding service at root, or connects
// it directly when root is empty, for a test fixture.
func forwardTo(t *testing.T, cfg *domain.Config, root string) {
	t.Helper()
	if err := cfg.SetForwardingRoot(ClientCodex, root); err != nil {
		t.Fatal(err)
	}
}

func forwardedConfig(t *testing.T) domain.Config {
	t.Helper()
	cfg := validConfig()
	forwardTo(t, &cfg, forwardingTestRoot)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the forwarded fixture is invalid: %v", err)
	}
	return cfg
}

// The destination follows the upstream: a moved provider endpoint, or a
// selection of another provider, keeps the forwarding with a destination
// that names the new upstream.
func TestForwardingFollowsAMovedUpstream(t *testing.T) {
	for name, test := range map[string]struct {
		move func(*domain.Config)
		want string
	}{
		"provider endpoint": {func(cfg *domain.Config) {
			account := cfg.Providers["backup"]
			account.Endpoints.Responses = "https://moved.test:8443/v2"
			cfg.Providers["backup"] = account
		}, forwardingTestRoot + "/https/moved.test:8443/v2"},
		"provider replaced": {func(cfg *domain.Config) {
			cfg.Providers["renamed"] = domain.Provider{Label: "Renamed", Endpoints: domain.Endpoints{Responses: "https://renamed.test/v1"}}
			delete(cfg.Providers, "backup")
			model := cfg.Models[domain.NewModelRef("backup", "gpt-test")]
			delete(cfg.Models, domain.NewModelRef("backup", "gpt-test"))
			model.Provider = "renamed"
			cfg.Models[model.Ref()] = model
			binding := cfg.Clients[ClientCodex]
			binding.Model = model.Ref()
			cfg.Clients[ClientCodex] = binding
		}, forwardingTestRoot + "/https/renamed.test/v1"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := forwardedConfig(t)
			test.move(&cfg)
			if err := cfg.Validate(); err != nil {
				t.Fatalf("a moved forwarded upstream was refused: %v", err)
			}
			if runtime, err := cfg.ResolveRuntime(ClientCodex, domain.ModelRef{}); err != nil || runtime.Endpoint != test.want {
				t.Fatalf("forwarding did not follow the moved upstream: %#v, %v", runtime, err)
			}
		})
	}
}

// A forwarded selection on another protocol than Responses, which the
// forwarding service does not relay, is refused with the step that connects
// the client directly.
func TestForwardingRefusesAProtocolChange(t *testing.T) {
	cfg := validConfig()
	cfg.Providers["multi"] = domain.Provider{Label: "Multi", Endpoints: domain.Endpoints{Responses: "https://multi.test/v1", Chat: "https://multi.test/v1"}}
	cfg.Models[domain.NewModelRef("multi", "multi-model")] = testModel("Multi", "multi", "multi-model", domain.ProtocolResponses, domain.ProtocolChat)
	cfg.SetSelectedModel(ClientHermes, domain.NewModelRef("multi", "multi-model"), domain.ProtocolResponses)
	if err := cfg.SetForwardingRoot(ClientHermes, forwardingTestRoot); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	// The same provider URL serves both protocols, so only the protocol moves.
	binding := cfg.Clients[ClientHermes]
	binding.Protocol = domain.ProtocolChat
	cfg.Clients[ClientHermes] = binding
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "`aigw client edit hermes --direct`") {
		t.Fatalf("a forwarded protocol change was accepted or not explained: %v", err)
	}
}

// A provider endpoint that the grammar cannot name is refused with the
// direct connection, and the retry budgets stay, because they belong to the
// provider.
func TestForwardingRefusalKeepsASelectedRetryPolicy(t *testing.T) {
	cfg := forwardedConfig(t)
	retries := uint64(2)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &retries}
	cfg.Clients[ClientCodex] = binding
	account := cfg.Providers["backup"]
	account.Endpoints.Responses = "https://moved.test/v1/./responses"
	cfg.Providers["backup"] = account
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "`aigw client edit codex --direct`") {
		t.Fatalf("the advised direct connection is not named: %v", err)
	}
	if cfg.Clients[ClientCodex].RetryPolicy == nil {
		t.Fatal("the refusal dropped the retry budgets")
	}
}

func TestForwardingSurvivesASelectionOfTheSameUpstream(t *testing.T) {
	cfg := forwardedConfig(t)
	cfg.Models[domain.NewModelRef("backup", "other-model")] = testModel("Other", "backup", "other-model", domain.ProtocolResponses)
	cfg.SetSelectedModel(ClientCodex, domain.NewModelRef("backup", "other-model"), "")
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if runtime, err := cfg.ResolveRuntime(ClientCodex, domain.ModelRef{}); err != nil || runtime.Endpoint != forwardingTestRoot+"/https/backup.test/v1" || runtime.UpstreamModel != "other-model" {
		t.Fatalf("a selection of the same upstream lost forwarding: %#v, %v", runtime, err)
	}
}

func TestDirectSelectionDropsForwarding(t *testing.T) {
	cfg := forwardedConfig(t)
	forwardTo(t, &cfg, "")
	if binding := cfg.Clients[ClientCodex]; binding.ForwardingRoot != "" {
		t.Fatalf("the direct selection kept the forwarding: %#v", binding)
	}
	account := cfg.Providers["backup"]
	account.Endpoints.Responses = "https://moved.test/v1?region=east"
	cfg.Providers["backup"] = account
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a provider edit after the direct selection was refused: %v", err)
	}
}

// TestForwardedCheckpointKeepsTheConfigurationThatWasChecked records the
// configuration that the check proved as the exact aigw.toml text, so that a
// forwarded client comes back with the root it was checked with.
func TestForwardedCheckpointKeepsTheConfigurationThatWasChecked(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := forwardedConfig(t)
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(context.Background(), cfg, sessions(ClientCodex), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.verifiedPath())
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Configuration string `json:"configuration"`
	}
	saved, readErr := os.ReadFile(store.Path())
	if err := json.Unmarshal(data, &document); err != nil || readErr != nil || !bytes.Equal([]byte(document.Configuration), saved) {
		t.Fatalf("the checkpoint does not hold the checked configuration: %s, %v, %v", data, err, readErr)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if restored, expected := checkpoint.Config.Clients[ClientCodex], cfg.Clients[ClientCodex]; restored.ForwardingRoot != expected.ForwardingRoot {
		t.Fatalf("the restored checkpoint lost its forwarding root: %#v, want %#v", restored, expected)
	}
}
