package store

import (
	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestForwardingChangedDuringVerificationRejectsCheckpoint(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := convergenceConfig("current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	if err := changed.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	data, err := encodeConfig(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err == nil {
		t.Fatal("verification certified a changed forwarding destination")
	}
	after, err := store.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected verification mutated its exact state: %v", err)
	}
}

func TestForwardingChangeDuringCheckpointWritePreservesNewerState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := convergenceConfig("current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	if err := changed.SetForwardingEndpoint(ClientCodex, "http://127.0.0.1:8792/v1"); err != nil {
		t.Fatal(err)
	}
	newer, err := encodeConfig(changed)
	if err != nil {
		t.Fatal(err)
	}
	originalWrite := writeConfigurationFileIfUnchanged
	t.Cleanup(func() { writeConfigurationFileIfUnchanged = originalWrite })
	writeConfigurationFileIfUnchanged = func(path string, expected platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
		postimage, err := originalWrite(path, expected, data, mode)
		if err == nil && path == store.verifiedPath() {
			err = os.WriteFile(store.Path(), newer, 0o600)
		}
		return postimage, err
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err == nil || !strings.Contains(err.Error(), "configuration changed while the verified snapshot was written") {
		t.Fatalf("concurrent forwarding update was certified: %v", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil || after.Verified.Exists || !bytes.Equal(after.Config.Data, newer) {
		t.Fatalf("checkpoint compensation lost newer forwarding or retained stale proof: %v", err)
	}
}

func TestVerifiedCheckpointRoundTripIsSecretFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path, testAdmission())
	cfg := admitted(domain.Config{
		Version:   domain.ConfigVersion,
		Providers: map[string]domain.Provider{"dmx": {Label: "DMX", Endpoints: domain.Endpoints{Anthropic: "https://example.test"}}},
		Models:    map[domain.ModelRef]domain.Model{domain.NewModelRef("dmx", "claude-test"): testModel("Claude", "dmx", "claude-test", domain.ProtocolAnthropic)},
		Clients:   map[string]domain.ClientBinding{ClientClaude: {Model: domain.NewModelRef("dmx", "claude-test")}},
	})
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions("claude", "codex")); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Config.SelectedModel(ClientClaude) != domain.NewModelRef("dmx", "claude-test") || len(checkpoint.Clients) != 2 || checkpoint.VerifiedAt.IsZero() {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
	data, err := os.ReadFile(NewStore(path, testAdmission()).verifiedPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "token") {
		t.Fatalf("checkpoint contains token-like content: %s", data)
	}
}

func TestVerifiedCheckpointRequiresOneCompleteDocument(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "configuration.toml"), testAdmission())
	cfg := convergenceConfig("verified")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.verifiedPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{" \n\t", "{}", "null", "truncated", `{"config":`} {
		t.Run(fmt.Sprintf("suffix=%q", suffix), func(t *testing.T) {
			if err := os.WriteFile(store.verifiedPath(), append(bytes.Clone(data), suffix...), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := store.LoadVerifiedCheckpoint()
			if (err == nil) != (strings.TrimSpace(suffix) == "") {
				t.Fatalf("checkpoint suffix %q: %v", suffix, err)
			}
		})
	}
}

func TestVerifiedCheckpointUsesOneClientScopeContract(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "configuration.toml"), testAdmission())
	cfg := convergenceConfig("verified")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err != nil {
		t.Fatal(err)
	}
	// The checkpoint as the file holds it, whose clients each case replaces.
	var checkpoint checkpointDocument
	if err := json.Unmarshal(readTestFile(t, store.verifiedPath()), &checkpoint); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeVerifiedCheckpoint(mustMarshal(t, checkpoint), testAdmission()); err != nil {
		t.Fatalf("the unchanged checkpoint does not read: %v", err)
	}
	// A client must be admitted, named once, and recorded at a depth of aigw
	// check.
	for _, clients := range [][]VerifiedClient{
		nil, {}, sessions(ClientCodex, ClientCodex), sessions("future"), sessions(ClientCodex, ""),
		{{Client: ClientCodex}}, {{Client: ClientCodex, Depth: "inference"}},
	} {
		t.Run(fmt.Sprint(clients), func(t *testing.T) {
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, clients); err == nil {
				t.Errorf("writer accepted invalid client scope: %v", clients)
			}
			after, err := store.CaptureSnapshot()
			if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
				t.Errorf("invalid scope changed persistence: %v", err)
			}
			invalid := checkpoint
			invalid.Clients = clients
			if _, err := decodeVerifiedCheckpoint(mustMarshal(t, invalid), testAdmission()); err == nil {
				t.Errorf("reader accepted invalid client scope: %v", clients)
			}
		})
	}
}

func TestVerifiedCheckpointPreservesNewerConfigurationAndCheckpoint(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	verified := convergenceConfig("verified")
	current := convergenceConfig("current")
	if err := store.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), current, sessions(ClientCodex)); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), verified, sessions(ClientCodex)); err == nil || !strings.Contains(err.Error(), "configuration changed") {
		t.Fatalf("stale verification was accepted: %v", err)
	}
	after, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
		t.Fatal("stale verification changed the current configuration or checkpoint")
	}
}

func TestVerifiedCheckpointRequiresExistingConfiguration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	if err := store.SaveVerifiedCheckpoint(t.Context(), convergenceConfig("missing"), sessions(ClientCodex)); err == nil {
		t.Fatal("verification created a checkpoint without configuration")
	}
	if _, err := os.Stat(store.verifiedPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("checkpoint exists after rejected verification: %v", err)
	}
}

func TestVerifiedCheckpointWaitsForMutationLockAndHonorsCancellation(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := convergenceConfig("current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	unlock, err := store.Lock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := store.SaveVerifiedCheckpoint(ctx, cfg, sessions(ClientCodex)); !errors.Is(err, context.Canceled) {
		t.Fatalf("checkpoint ignored mutation lock or cancellation: %v", err)
	}
	if _, err := os.Stat(store.verifiedPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled checkpoint was written: %v", err)
	}
}

func TestVerifiedCheckpointAcceptsEquivalentConfigurationFormatting(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := convergenceConfig("current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte("# operator annotation\n\n"), data...)
	if err := os.WriteFile(store.Path(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(store.Path())
	if err != nil || !bytes.Equal(current, data) {
		t.Fatalf("verification reformatted operator configuration: %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsLegacyProfileOwnedEndpointResidue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	checkpoint := `{
  "config": {
    "version": 1,
    "accounts": {
      "gateway": {
        "label": "Gateway",
        "endpoints": {"openai_responses": "https://gateway.test/v1"}
      }
    },
    "profiles": {
      "gpt": {
        "label": "GPT",
        "account": "gateway",
        "client": "codex",
        "models": {"codex": "gpt-test"},
        "endpoints": {"openai_responses": "https://duplicate.test/v1"}
      }
    },
    "routes": {"default": "gpt"}
  },
  "clients": [{"client": "codex", "depth": "client"}],
  "verified_at": "2026-07-11T00:00:00Z"
}`
	if err := os.WriteFile(NewStore(path, testAdmission()).verifiedPath(), []byte(checkpoint), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path, testAdmission()).LoadVerifiedCheckpoint(); err == nil {
		t.Fatal("Legacy Profile-owned checkpoint endpoint residue was accepted")
	}
}

func TestSaveVerifiedCheckpointRejectsInvalidConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := NewStore(path, testAdmission()).SaveVerifiedCheckpoint(t.Context(), domain.Config{}, sessions("codex")); err == nil {
		t.Fatal("SaveVerifiedCheckpoint accepted an invalid configuration")
	}
}

func TestSaveVerifiedCheckpointSurfacesUnwritableConfigDirectory(t *testing.T) {
	// See TestLockSurfacesUnwritableConfigDirectory: a file blocking the
	// directory component is reachable on every platform, unlike chmod.
	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(blocked, "child", " toml"), testAdmission())
	if err := store.SaveVerifiedCheckpoint(t.Context(), convergenceConfig("current"), sessions("codex")); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an unwritable config directory")
	}
}

func TestLoadVerifiedCheckpointSurfacesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := NewStore(path, testAdmission()).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "read verified checkpoint") {
		t.Fatalf("missing checkpoint error = %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsIncompleteCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(NewStore(path, testAdmission()).verifiedPath(), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path, testAdmission()).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete checkpoint error = %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsNonCanonicalConfigVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	checkpoint := `{
  "configuration": "version = 7\n",
  "clients": [{"client": "codex", "depth": "client"}],
  "verified_at": "2026-07-11T00:00:00Z"
}`
	if err := os.WriteFile(NewStore(path, testAdmission()).verifiedPath(), []byte(checkpoint), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path, testAdmission()).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "validate verified checkpoint") {
		t.Fatalf("non-canonical config version error = %v", err)
	}
}

func TestSaveVerifiedCheckpointSurfacesInvalidCheckpointTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path, testAdmission())
	cfg := convergenceConfig("current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(NewStore(path, testAdmission()).verifiedPath(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions("codex")); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an existing directory at the checkpoint path")
	}
}

func TestVerifiedCheckpointCompensatesConfigurationChangeDuringWrite(t *testing.T) {
	for _, replaceCheckpoint := range []bool{false, true} {
		t.Run(fmt.Sprint(replaceCheckpoint), func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			cfg := convergenceConfig("verified")
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			current, err := encodeConfig(convergenceConfig("current"))
			if err != nil {
				t.Fatal(err)
			}
			originalWrite := writeConfigurationFileIfUnchanged
			t.Cleanup(func() { writeConfigurationFileIfUnchanged = originalWrite })
			writeConfigurationFileIfUnchanged = func(path string, before platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
				postimage, err := originalWrite(path, before, data, mode)
				if err != nil {
					return platform.FileSnapshot{}, err
				}
				if err := os.WriteFile(store.Path(), current, 0o600); err != nil {
					t.Fatal(err)
				}
				if replaceCheckpoint {
					if err := os.WriteFile(path, []byte("newer checkpoint"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return postimage, nil
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err == nil || !strings.Contains(err.Error(), "configuration changed") {
				t.Fatalf("changed configuration was certified: %v", err)
			}
			data, err := os.ReadFile(store.verifiedPath())
			if replaceCheckpoint {
				if err != nil || string(data) != "newer checkpoint" {
					t.Fatalf("newer checkpoint was changed: %q, %v", data, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("stale checkpoint remained: %q, %v", data, err)
			}
			data, err = os.ReadFile(store.Path())
			if err != nil || !bytes.Equal(data, current) {
				t.Fatalf("newer configuration was changed: %q, %v", data, err)
			}
		})
	}
}

// sessions records clients, each of which completed a session, in a verified
// snapshot.
func sessions(clients ...string) []VerifiedClient {
	verified := make([]VerifiedClient, 0, len(clients))
	for _, client := range clients {
		verified = append(verified, VerifiedClient{Client: client, Depth: "client"})
	}
	return verified
}

// A check whose configuration file no longer reads writes no snapshot.
func TestSaveVerifiedCheckpointRefusesAConfigurationThatNoLongerReads(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "aigw.toml"), testAdmission())
	cfg := convergenceConfig("current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte("version = 8\n[[providers]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, sessions(ClientCodex)); err == nil {
		t.Fatal("a snapshot of a configuration that no longer reads was written")
	}
	if _, err := os.Stat(store.verifiedPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a snapshot exists: %v", err)
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
