package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCaptureVerifiedBackupStateRequiresExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_, err := NewStore(path).CaptureVerifiedBackupState()
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing config error = %v", err)
	}
}

func TestCaptureVerifiedBackupStateSurfacesConfigReadErrors(t *testing.T) {
	path := t.TempDir()
	if _, err := NewStore(path).CaptureVerifiedBackupState(); err == nil {
		t.Fatal("CaptureVerifiedBackupState succeeded despite a directory at the config path")
	}
}

func TestCaptureVerifiedBackupStateSurfacesBackupReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".bak", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureVerifiedBackupState(); err == nil {
		t.Fatal("CaptureVerifiedBackupState succeeded despite a directory at the backup path")
	}
}

func TestCaptureVerifiedBackupStateSurfacesVerifiedReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureVerifiedBackupState(); err == nil {
		t.Fatal("CaptureVerifiedBackupState succeeded despite a directory at the verified checkpoint path")
	}
}

func TestCaptureVerifiedBackupStateSurfacesConfigDecodeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("not = [valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".verified.json", []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(path).CaptureVerifiedBackupState()
	if err == nil || !strings.Contains(err.Error(), "decode current config snapshot") {
		t.Fatalf("malformed config decode error = %v", err)
	}
}

func TestCaptureVerifiedBackupStateSurfacesCheckpointDecodeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".verified.json", []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CaptureVerifiedBackupState(); err == nil || !strings.Contains(err.Error(), "parse verified checkpoint") {
		t.Fatalf("malformed checkpoint decode error = %v", err)
	}
}

func TestConvergeVerifiedBackupSurfacesConfigReadErrors(t *testing.T) {
	path := t.TempDir()
	err := NewStore(path).ConvergeVerifiedBackup(Snapshot{})
	if err == nil {
		t.Fatal("ConvergeVerifiedBackup succeeded despite a directory at the config path")
	}
}

func TestConvergeVerifiedBackupSurfacesVerifiedReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	configSnapshot, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".verified.json", 0o700); err != nil {
		t.Fatal(err)
	}
	expected := Snapshot{Config: configSnapshot.Config}
	if err := store.ConvergeVerifiedBackup(expected); err == nil {
		t.Fatal("ConvergeVerifiedBackup succeeded despite a directory at the verified checkpoint path")
	}
}

func TestLoadBackupRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("old")); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	backup, err := store.LoadBackup()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := backup.Accounts["old"]; !ok {
		t.Fatalf("backup config = %#v, want the previous version", backup)
	}
}

func TestLoadBackupSurfacesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := NewStore(path).LoadBackup(); err == nil || !strings.Contains(err.Error(), "read previous config backup") {
		t.Fatalf("missing backup error = %v", err)
	}
}

func TestCaptureVerifiedBackupStateBindsCurrentConfiguration(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("configuration-changed=%t", changed), func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
			current := convergenceConfig("current")
			if err := store.Save(current); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), current, AdmittedClientIDs()); err != nil {
				t.Fatal(err)
			}
			if changed {
				profile := current.Profiles["current"]
				profile.Model = "unverified-model"
				current.Profiles["current"] = profile
			}
			data, err := encodeConfig(current)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.Path(), append([]byte("# user formatting\n"), data...), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			state, err := store.CaptureVerifiedBackupState()
			if changed {
				if err == nil || !strings.Contains(err.Error(), "does not match current configuration") {
					t.Fatalf("stale checkpoint admitted: %v", err)
				}
			} else if err != nil || !state.Snapshot.Config.Equal(before.Config) || state.Current.Profiles["current"].Model != current.Profiles["current"].Model {
				t.Fatalf("matching checkpoint rejected or captured different bytes: %v", err)
			}
			after, err := store.CaptureSnapshot()
			if err != nil || !before.Config.Equal(after.Config) || !before.Backup.Equal(after.Backup) || !before.Verified.Equal(after.Verified) {
				t.Fatalf("checkpoint observation changed persistence: %v", err)
			}
		})
	}
}

func TestConvergeVerifiedBackupCopiesExactCurrentBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	oldConfig := convergenceConfig("old")
	currentConfig := convergenceConfig("current")
	if err := store.Save(oldConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(currentConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVerifiedCheckpoint(t.Context(), currentConfig, AdmittedClientIDs()); err != nil {
		t.Fatal(err)
	}
	currentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	customBytes := append([]byte("# byte-exact verified current\n"), currentBytes...)
	if err := os.WriteFile(path, customBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path+".bak", 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := store.CaptureVerifiedBackupState()
	if err != nil {
		t.Fatal(err)
	}
	verifiedBefore, err := os.ReadFile(path + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.ConvergeVerifiedBackup(state.Snapshot); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, customBytes) || !bytes.Equal(result.Backup.Data, customBytes) {
		t.Fatalf("backup was not converged byte-exactly\nwant %q\ngot  %q", customBytes, backup)
	}
	if want := securePersistedFileMode(); result.Backup.Mode != want {
		t.Fatalf("converged backup mode = %o, want %o", result.Backup.Mode, want)
	}
	verifiedAfter, err := os.ReadFile(path + ".verified.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(verifiedAfter, verifiedBefore) {
		t.Fatal("backup convergence changed the verified checkpoint")
	}
}

func TestConvergeVerifiedBackupRejectsChangedPreimages(t *testing.T) {
	for _, test := range []struct {
		name           string
		suffix         string
		appendix       string
		appendOriginal bool
	}{
		{"config", "", "# external change\n", true},
		{"backup", ".bak", "external backup change\n", false},
		{"verified", ".verified.json", "\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			store := NewStore(path)
			if err := store.Save(convergenceConfig("old")); err != nil {
				t.Fatal(err)
			}
			current := convergenceConfig("current")
			if err := store.Save(current); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), current, AdmittedClientIDs()); err != nil {
				t.Fatal(err)
			}
			state, err := store.CaptureVerifiedBackupState()
			if err != nil {
				t.Fatal(err)
			}
			changedPath := path + test.suffix
			original, err := os.ReadFile(changedPath)
			if err != nil {
				t.Fatal(err)
			}
			changedBytes := []byte(test.appendix)
			if test.appendOriginal {
				changedBytes = append(original, changedBytes...)
			}
			if err := os.WriteFile(changedPath, changedBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			want, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}

			if err := store.ConvergeVerifiedBackup(state.Snapshot); err == nil || !strings.Contains(err.Error(), "preimage changed") {
				t.Fatalf("convergence error = %v", err)
			}
			got, err := store.CaptureSnapshot()
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("owned files changed after %s preimage conflict: got=%+v want=%+v error=%v", test.name, got, want, err)
			}
		})
	}
}

func TestCaptureVerifiedBackupStateRequiresCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	store := NewStore(path)
	if err := store.Save(convergenceConfig("current")); err != nil {
		t.Fatal(err)
	}
	_, err := store.CaptureVerifiedBackupState()
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing checkpoint error = %v", err)
	}
}

func convergenceConfig(id string) Config {
	cfg := NewConfig()
	cfg.Accounts[id] = Account{Label: strings.ToUpper(id), Endpoints: Endpoints{OpenAIResponses: "https://" + id + ".test/v1"}}
	cfg.Profiles[id] = Profile{Label: strings.ToUpper(id), Account: id, Client: ClientCodex, Model: id + "-model"}
	cfg.Routes[ClientCodex] = id
	return cfg
}
