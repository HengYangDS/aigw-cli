package configuration

import (
	"aigw-cli/internal/transaction"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifiedCheckpointRoundTripIsSecretFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	cfg := Config{
		Version:  ConfigVersion,
		Accounts: map[string]Account{"dmx": {Label: "DMX", Endpoints: Endpoints{Anthropic: "https://example.test"}}},
		Profiles: map[string]Profile{"claude": {Label: "Claude", Account: "dmx", Client: ClientClaude, Model: "claude-test"}},
		Routes:   Routes{ClientClaude: "claude"},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{"claude", "codex"}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Config.Routes[ClientClaude] != "claude" || len(checkpoint.Clients) != 2 || checkpoint.VerifiedAt.IsZero() {
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

func TestVerifiedCheckpointRequiresOneCompleteDocument(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := convergenceConfig("verified")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.Path() + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{" \n\t", "{}", "null", "truncated", `{"config":`} {
		t.Run(fmt.Sprintf("suffix=%q", suffix), func(t *testing.T) {
			if err := os.WriteFile(store.Path()+".verified.json", append(bytes.Clone(data), suffix...), 0o600); err != nil {
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
	store := NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := convergenceConfig("verified")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := store.LoadVerifiedCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	for _, clients := range [][]string{nil, {}, {ClientCodex, ClientCodex}, {"future"}, {ClientCodex, ""}} {
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
			data, err := json.Marshal(invalid)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeVerifiedCheckpoint(data); err == nil {
				t.Errorf("reader accepted invalid client scope: %v", clients)
			}
		})
	}
}

func TestVerifiedCheckpointPreservesNewerConfigurationAndCheckpoint(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	verified := convergenceConfig("verified")
	current := convergenceConfig("current")
	if err := store.Save(current); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), current, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), verified, []string{ClientCodex}); err == nil || !strings.Contains(err.Error(), "configuration changed") {
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
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
	if err := store.SaveVerifiedCheckpoint(t.Context(), convergenceConfig("missing"), []string{ClientCodex}); err == nil {
		t.Fatal("verification created a checkpoint without configuration")
	}
	if _, err := os.Stat(store.Path() + ".verified.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("checkpoint exists after rejected verification: %v", err)
	}
}

func TestVerifiedCheckpointWaitsForMutationLockAndHonorsCancellation(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
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
	if err := store.SaveVerifiedCheckpoint(ctx, cfg, []string{ClientCodex}); !errors.Is(err, context.Canceled) {
		t.Fatalf("checkpoint ignored mutation lock or cancellation: %v", err)
	}
	if _, err := os.Stat(store.Path() + ".verified.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled checkpoint was written: %v", err)
	}
}

func TestVerifiedCheckpointAcceptsEquivalentConfigurationFormatting(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
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
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(store.Path())
	if err != nil || !bytes.Equal(current, data) {
		t.Fatalf("verification reformatted operator configuration: %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsProfileOwnedEndpointResidue(t *testing.T) {
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
  "clients": ["codex"],
  "verified_at": "2026-07-11T00:00:00Z"
}`
	if err := os.WriteFile(path+".verified.json", []byte(checkpoint), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil {
		t.Fatal("Profile-owned checkpoint endpoint residue was accepted")
	}
}

func TestSaveVerifiedCheckpointRejectsInvalidConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := NewStore(path).SaveVerifiedCheckpoint(t.Context(), Config{}, []string{"codex"}); err == nil {
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
	store := NewStore(filepath.Join(blocked, "child", " toml"))
	if err := store.SaveVerifiedCheckpoint(t.Context(), convergenceConfig("current"), []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an unwritable config directory")
	}
}

func TestLoadVerifiedCheckpointSurfacesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "read verified checkpoint") {
		t.Fatalf("missing checkpoint error = %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsIncompleteCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path+".verified.json", []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete checkpoint error = %v", err)
	}
}

func TestLoadVerifiedCheckpointRejectsNonCanonicalConfigVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	checkpoint := `{
  "config": {
    "version": 1,
    "accounts": {"team": {"label": "Team", "endpoints": {"anthropic": "https://team.test"}}},
    "profiles": {"team": {"label": "Team", "account": "team"}},
    "routes": {"default": "team"}
  },
  "clients": ["codex"],
  "verified_at": "2026-07-11T00:00:00Z"
}`
	if err := os.WriteFile(path+".verified.json", []byte(checkpoint), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).LoadVerifiedCheckpoint(); err == nil || !strings.Contains(err.Error(), "validate verified checkpoint") {
		t.Fatalf("non-canonical config version error = %v", err)
	}
}

func TestSaveVerifiedCheckpointSurfacesInvalidCheckpointTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	cfg := convergenceConfig("current")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{"codex"}); err == nil {
		t.Fatal("SaveVerifiedCheckpoint succeeded despite an existing directory at the checkpoint path")
	}
}

func TestVerifiedCheckpointCompensatesConfigurationChangeDuringWrite(t *testing.T) {
	for _, replaceCheckpoint := range []bool{false, true} {
		t.Run(fmt.Sprint(replaceCheckpoint), func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"))
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
			writeConfigurationFileIfUnchanged = func(path string, before transaction.FileSnapshot, data []byte, mode os.FileMode) (transaction.FileSnapshot, error) {
				postimage, err := originalWrite(path, before, data, mode)
				if err != nil {
					return transaction.FileSnapshot{}, err
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
			if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err == nil || !strings.Contains(err.Error(), "configuration changed") {
				t.Fatalf("changed configuration was certified: %v", err)
			}
			data, err := os.ReadFile(store.Path() + ".verified.json")
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
