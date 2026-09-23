package synchronization

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	changed, _, err := syncer.SelectRoute(t.Context(), before, configuration.ClientClaude, "next", "")
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
	changed, _, err = syncer.SelectRoute(t.Context(), current, configuration.ClientClaude, "next", "")
	if err != nil || changed || credentials.writes != 0 {
		t.Fatalf("repeated selection = %t, %v; credential writes=%d", changed, err, credentials.writes)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !snapshot.Config.Equal(after.Config) || !snapshot.Backup.Equal(after.Backup) || !snapshot.Verified.Equal(after.Verified) {
		t.Fatalf("repeated selection changed persistence: %v", err)
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
			syncer := Synchronizer{Config: store, Secrets: credentials, Discovery: setupDiscovery(nil)}
			switch phase {
			case "cancelled":
				cancel()
				failure = context.Canceled
			case "discovery":
				syncer.Discovery = setupDiscovery(cancel)
				failure = context.Canceled
			}
			_, _, err := syncer.SelectRoute(ctx, before, configuration.ClientClaude, "claude", "new-token")
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
			_, _, err := (Synchronizer{Config: store, Secrets: credentials}).SelectRoute(t.Context(), cfg, client, route, "token")
			if err == nil || credentials.writes != 0 {
				t.Fatalf("invalid selection reached credential mutation: writes=%d, error=%v", credentials.writes, err)
			}
			if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
				t.Fatalf("invalid selection created configuration: %v", err)
			}
		})
	}
}
