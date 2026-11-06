package synchronization

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

func TestModelSelectionOwnsPersistenceAndRepeatedSelection(t *testing.T) {
	before := setupConfiguration()
	before.Models["next"] = domain.Model{Label: "Next", Provider: "team", Family: "claude-next", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {}}}
	configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
	if err := configStore.Save(before); err != nil {
		t.Fatal(err)
	}
	credentials := setupTokenStore(t, "existing-token")
	syncer := Synchronizer{Registry: builtinClients, Config: configStore, Secrets: credentials, Discovery: setupDiscovery(nil)}
	changed, _, _, err := syncer.SelectModel(t.Context(), before, claude.ID, "next", "", domain.ConnectionChange{}, "")
	if err != nil || !changed {
		t.Fatalf("selection = %t, %v; want committed change", changed, err)
	}
	current, err := configStore.Load()
	if err != nil || current.SelectedModel(claude.ID) != "next" || before.SelectedModel(claude.ID) != "claude" {
		t.Fatalf("selection mutated its input or failed persistence: %v", err)
	}
	snapshot, err := configStore.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	changed, _, _, err = syncer.SelectModel(t.Context(), current, claude.ID, "next", "", domain.ConnectionChange{}, "")
	if err != nil || changed || credentials.writes != 0 {
		t.Fatalf("repeated selection = %t, %v; credential writes=%d", changed, err, credentials.writes)
	}
	after, err := configStore.CaptureSnapshot()
	if err != nil || !snapshot.Config.Equal(after.Config) || !snapshot.Backup.Equal(after.Backup) || !snapshot.Verified.Equal(after.Verified) {
		t.Fatalf("repeated selection changed persistence: %v", err)
	}
}

func TestModelSelectionResolvesProtocolBeforeForwardingIdentity(t *testing.T) {
	direct := ""
	for _, test := range []struct {
		name, route, wantEndpoint string
		protocol                  domain.EndpointProtocol
		forwarding                *string
	}{
		{name: "same upstream", route: "dual", protocol: domain.ProtocolOpenAIChatCompletions, wantEndpoint: "http://127.0.0.1:8792/selected/v1"},
		{name: "explicit direct", route: "dual", protocol: domain.ProtocolOpenAIChatCompletions, forwarding: &direct, wantEndpoint: "https://team.test/v1"},
		{name: "new protocol", route: "dual", protocol: domain.ProtocolOpenAIResponses, wantEndpoint: "https://team.test/responses/v1"},
		{name: "new Account", route: "other", protocol: domain.ProtocolOpenAIChatCompletions, wantEndpoint: "https://other.test/v1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := setupConfiguration()
			cfg.Providers["team"] = domain.Provider{Label: "Team", Endpoints: domain.Endpoints{
				Anthropic: "https://team.test", OpenAIChatCompletions: "https://team.test/v1", OpenAIResponses: "https://team.test/responses/v1",
			}}
			cfg.Providers["other"] = domain.Provider{Label: "Other", Endpoints: domain.Endpoints{OpenAIChatCompletions: "https://other.test/v1"}}
			cfg.Models["chat"] = domain.Model{Label: "Chat", Provider: "team", Family: "shared", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}}}
			cfg.Models["dual"] = domain.Model{Label: "Dual", Provider: "team", Family: "shared", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}, domain.ProtocolOpenAIResponses: {}}}
			cfg.Models["other"] = domain.Model{Label: "Other", Provider: "other", Family: "shared", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIChatCompletions: {}}}
			cfg.Clients[hermes.ID] = domain.ClientBinding{Model: "chat", Enabled: true}
			if err := cfg.SetForwardingEndpoint(hermes.ID, "http://127.0.0.1:8792/selected/v1"); err != nil {
				t.Fatal(err)
			}
			binding := cfg.Clients[hermes.ID]
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
			proposed, selected, err := (Synchronizer{Registry: builtinClients}).PrepareSelection(cfg, hermes.ID, test.route, test.protocol, domain.ConnectionChange{ForwardingEndpoint: test.forwarding})
			if err != nil {
				t.Fatal(err)
			}
			if selected.Protocol != test.protocol || selected.Endpoint != test.wantEndpoint {
				t.Fatalf("selection = %s %s, want %s %s", selected.Protocol, selected.Endpoint, test.protocol, test.wantEndpoint)
			}
			if proposed.Clients[hermes.ID].Model != test.route || !reflect.DeepEqual(cfg.Clients[hermes.ID], binding) {
				t.Fatal("selection lost its Route or mutated the original binding")
			}
		})
	}
}

func TestModelSelectionCompensatesCredentialsBeforeCommit(t *testing.T) {
	for _, phase := range []string{"cancelled", "discovery", "persistence"} {
		t.Run(phase, func(t *testing.T) {
			before := setupConfiguration()
			delete(before.Clients, claude.ID)
			credentials := secret.NewMemoryStore()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			failure := errors.New("configuration write failed")
			store := &configStoreStub{commitErr: failure}
			store.bindConfiguration(t, before)
			syncer := Synchronizer{Registry: builtinClients, Config: store, Secrets: credentials, Discovery: setupDiscovery(nil)}
			switch phase {
			case "cancelled":
				cancel()
				failure = context.Canceled
			case "discovery":
				syncer.Discovery = setupDiscovery(cancel)
				failure = context.Canceled
			}
			_, _, _, err := syncer.SelectModel(ctx, before, claude.ID, "claude", "", domain.ConnectionChange{}, "new-token")
			if !errors.Is(err, failure) {
				t.Fatalf("selection error = %v, want %v", err, failure)
			}
			if token, err := credentials.Get("team"); !errors.Is(err, secret.ErrNotFound) || token != "" {
				t.Fatalf("failed selection retained credential: %v", err)
			}
			if before.SelectedModel(claude.ID) != "" || (phase != "persistence" && store.commits != 0) {
				t.Fatal("failed selection modified its configuration input or committed after cancellation")
			}
		})
	}
}

func TestModelSelectionValidatesOwnershipBeforeTokenWrites(t *testing.T) {
	for _, route := range []string{"unknown", "native"} {
		t.Run(route, func(t *testing.T) {
			cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
			cfg.Models["native"] = domain.Model{Label: "Native", Provider: "gateway", Family: "model", Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}}}
			binding := cfg.Clients[codex.ID]
			binding.ModelProvider = "provider"
			binding.Authentication = domain.AuthenticationClientNative
			cfg.Clients[codex.ID] = binding
			configStore := store.NewStore(filepath.Join(t.TempDir(), "aigw.toml"), clients.Admission())
			credentials := &setupCredentials{Store: secret.NewMemoryStore()}
			client := claude.ID
			if route == "native" {
				client = codex.ID
			}
			_, _, _, err := (Synchronizer{Registry: builtinClients, Config: configStore, Secrets: credentials}).SelectModel(t.Context(), cfg, client, route, "", domain.ConnectionChange{}, "token")
			if err == nil || credentials.writes != 0 {
				t.Fatalf("invalid selection reached credential mutation: writes=%d, error=%v", credentials.writes, err)
			}
			if _, err := os.Stat(configStore.Path()); !os.IsNotExist(err) {
				t.Fatalf("invalid selection created configuration: %v", err)
			}
		})
	}
}
