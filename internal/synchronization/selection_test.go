package synchronization

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestRouteSelectionOwnsPersistenceAndRepeatedSelection(t *testing.T) {
	before := setupConfiguration()
	before.Routes["next"] = configuration.Route{Label: "Next", Account: "team", Model: "claude-next", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolAnthropic: {}}}
	store := configuration.NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	credentials := setupTokenStore(t, "existing-token")
	syncer := Synchronizer{Config: store, Secrets: credentials, Discovery: setupDiscovery(nil)}
	changed, _, _, err := syncer.SelectRoute(t.Context(), before, configuration.ClientClaude, "next", "", nil, "")
	if err != nil || !changed {
		t.Fatalf("selection = %t, %v; want committed change", changed, err)
	}
	current, err := store.Load()
	if err != nil || current.SelectedRoute(configuration.ClientClaude) != "next" || before.SelectedRoute(configuration.ClientClaude) != "claude" {
		t.Fatalf("selection mutated its input or failed persistence: %v", err)
	}
	snapshot, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	changed, _, _, err = syncer.SelectRoute(t.Context(), current, configuration.ClientClaude, "next", "", nil, "")
	if err != nil || changed || credentials.writes != 0 {
		t.Fatalf("repeated selection = %t, %v; credential writes=%d", changed, err, credentials.writes)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !snapshot.Config.Equal(after.Config) || !snapshot.Backup.Equal(after.Backup) || !snapshot.Verified.Equal(after.Verified) {
		t.Fatalf("repeated selection changed persistence: %v", err)
	}
}

func TestRouteSelectionResolvesProtocolBeforeForwardingIdentity(t *testing.T) {
	direct := ""
	for _, test := range []struct {
		name, route, wantEndpoint string
		protocol                  configuration.EndpointProtocol
		forwarding                *string
	}{
		{name: "same upstream", route: "dual", protocol: configuration.ProtocolOpenAIChatCompletions, wantEndpoint: "http://127.0.0.1:8792/selected/v1"},
		{name: "explicit direct", route: "dual", protocol: configuration.ProtocolOpenAIChatCompletions, forwarding: &direct, wantEndpoint: "https://team.test/v1"},
		{name: "new protocol", route: "dual", protocol: configuration.ProtocolOpenAIResponses, wantEndpoint: "https://team.test/responses/v1"},
		{name: "new Account", route: "other", protocol: configuration.ProtocolOpenAIChatCompletions, wantEndpoint: "https://other.test/v1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := setupConfiguration()
			cfg.Accounts["team"] = configuration.Account{Label: "Team", Endpoints: configuration.Endpoints{
				Anthropic: "https://team.test", OpenAIChatCompletions: "https://team.test/v1", OpenAIResponses: "https://team.test/responses/v1",
			}}
			cfg.Accounts["other"] = configuration.Account{Label: "Other", Endpoints: configuration.Endpoints{OpenAIChatCompletions: "https://other.test/v1"}}
			cfg.Routes["chat"] = configuration.Route{Label: "Chat", Account: "team", Model: "shared", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIChatCompletions: {}}}
			cfg.Routes["dual"] = configuration.Route{Label: "Dual", Account: "team", Model: "shared", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIChatCompletions: {}, configuration.ProtocolOpenAIResponses: {}}}
			cfg.Routes["other"] = configuration.Route{Label: "Other", Account: "other", Model: "shared", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIChatCompletions: {}}}
			binding := configuration.ClientBinding{Route: "chat", Enabled: true, ForwardingEndpoint: "http://127.0.0.1:8792/selected/v1"}
			cfg.Clients[configuration.ClientHermes] = binding
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
			proposed, selected, err := (Synchronizer{}).PrepareSelection(cfg, configuration.ClientHermes, test.route, test.protocol, test.forwarding)
			if err != nil {
				t.Fatal(err)
			}
			if selected.Protocol != test.protocol || selected.Endpoint != test.wantEndpoint {
				t.Fatalf("selection = %s %s, want %s %s", selected.Protocol, selected.Endpoint, test.protocol, test.wantEndpoint)
			}
			if proposed.Clients[configuration.ClientHermes].Route != test.route || !reflect.DeepEqual(cfg.Clients[configuration.ClientHermes], binding) {
				t.Fatal("selection lost its Route or mutated the original binding")
			}
		})
	}
}

func TestRouteSelectionCompensatesCredentialsBeforeCommit(t *testing.T) {
	for _, phase := range []string{"cancelled", "discovery", "persistence"} {
		t.Run(phase, func(t *testing.T) {
			before := setupConfiguration()
			delete(before.Clients, configuration.ClientClaude)
			credentials := secrets.NewMemoryStore()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			failure := errors.New("configuration write failed")
			store := &configStoreStub{commitErr: failure}
			store.bindConfiguration(t, before)
			syncer := Synchronizer{Config: store, Secrets: credentials, Discovery: setupDiscovery(nil)}
			switch phase {
			case "cancelled":
				cancel()
				failure = context.Canceled
			case "discovery":
				syncer.Discovery = setupDiscovery(cancel)
				failure = context.Canceled
			}
			_, _, _, err := syncer.SelectRoute(ctx, before, configuration.ClientClaude, "claude", "", nil, "new-token")
			if !errors.Is(err, failure) {
				t.Fatalf("selection error = %v, want %v", err, failure)
			}
			if token, err := credentials.Get("team"); !errors.Is(err, secrets.ErrNotFound) || token != "" {
				t.Fatalf("failed selection retained credential: %v", err)
			}
			if before.SelectedRoute(configuration.ClientClaude) != "" || (phase != "persistence" && store.commits != 0) {
				t.Fatal("failed selection modified its configuration input or committed after cancellation")
			}
		})
	}
}

func TestRouteSelectionValidatesOwnershipBeforeTokenWrites(t *testing.T) {
	for _, route := range []string{"unknown", "native"} {
		t.Run(route, func(t *testing.T) {
			cfg := testConfig(filepath.Join(t.TempDir(), "config.toml"))
			cfg.Routes["native"] = configuration.Route{Label: "Native", Account: "gateway", Model: "model", Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}}}
			binding := cfg.Clients[configuration.ClientCodex]
			binding.ModelProvider = "provider"
			binding.Authentication = configuration.AuthenticationClientNative
			cfg.Clients[configuration.ClientCodex] = binding
			store := configuration.NewStore(filepath.Join(t.TempDir(), "aigw.toml"))
			credentials := &setupCredentials{Store: secrets.NewMemoryStore()}
			client := configuration.ClientClaude
			if route == "native" {
				client = configuration.ClientCodex
			}
			_, _, _, err := (Synchronizer{Config: store, Secrets: credentials}).SelectRoute(t.Context(), cfg, client, route, "", nil, "token")
			if err == nil || credentials.writes != 0 {
				t.Fatalf("invalid selection reached credential mutation: writes=%d, error=%v", credentials.writes, err)
			}
			if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
				t.Fatalf("invalid selection created configuration: %v", err)
			}
		})
	}
}
