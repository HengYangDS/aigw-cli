package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

func TestUnreadableCapabilityComponentStopsOnlyFullReads(t *testing.T) {
	for _, suffix := range []string{"", ".bak"} {
		t.Run("component"+suffix, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			cfg := toolSearchConfig()
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientClaude}); err != nil {
				t.Fatal(err)
			}
			state, err := store.CaptureVerifiedBackupState()
			if err != nil {
				t.Fatal(err)
			}
			if err := store.ConvergeVerifiedBackup(state.Snapshot); err != nil {
				t.Fatal(err)
			}
			path := capabilityComponentPath(store) + suffix
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CaptureSnapshot(); err == nil {
				t.Fatal("snapshot capture ignored an unreadable capability component")
			}
			if _, err := store.LoadCredentialConfig(); err != nil {
				t.Fatalf("credential configuration depends on the capability component: %v", err)
			}
			_, loadErr := store.Load()
			_, backupErr := store.LoadBackup()
			_, checkpointErr := store.LoadVerifiedCheckpoint()
			saveErr := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientClaude})
			current := suffix == ""
			if (loadErr != nil) != current || (checkpointErr != nil) != current || (saveErr != nil) != current || (backupErr != nil) == current {
				t.Fatalf("unreadable %q: load=%v checkpoint=%v save=%v backup=%v", suffix, loadErr, checkpointErr, saveErr, backupErr)
			}
		})
	}
}

func TestCapabilityComponentWriteFailureRestoresOwnedFiles(t *testing.T) {
	for _, suffix := range []string{"", ".bak"} {
		t.Run("component"+suffix, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			cfg := toolSearchConfig()
			if err := store.Save(cfg); err != nil {
				t.Fatal(err)
			}
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			failure := failCapabilityWrites(t, capabilityComponentPath(store)+suffix)
			changed := cfg.Clone()
			account := changed.Accounts["ucloud"]
			account.Label = "UCloud Updated"
			changed.Accounts["ucloud"] = account
			if _, err := store.Commit(before, changed); !errors.Is(err, failure) {
				t.Fatalf("failed capability write = %v", err)
			}
			if after, err := store.CaptureSnapshot(); err != nil || !before.equal(after) {
				t.Fatalf("failed capability write did not restore every owned file: %v", err)
			}
		})
	}
}

func TestVerifiedBackupConvergenceGuardsTheCapabilityComponent(t *testing.T) {
	for _, change := range []string{"preimage", "write"} {
		t.Run(change, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
			if err := store.Save(toolSearchConfig()); err != nil {
				t.Fatal(err)
			}
			state, err := store.CaptureVerifiedBackupState()
			if err != nil {
				t.Fatal(err)
			}
			want := "route capability preimage changed"
			if change == "preimage" {
				if err := os.WriteFile(capabilityComponentPath(store), []byte("# newer operator content\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				want = failCapabilityWrites(t, capabilityComponentPath(store)+".bak").Error()
			}
			if err := store.ConvergeVerifiedBackup(state.Snapshot); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("backup convergence error = %v, want %q", err, want)
			}
			if _, err := os.Stat(capabilityComponentPath(store) + ".bak"); !os.IsNotExist(err) {
				t.Fatalf("refused convergence left a capability backup: %v", err)
			}
		})
	}
}

func TestRestoreSnapshotReportsCapabilityComponentConflicts(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	cfg := toolSearchConfig()
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	changed := cfg.Clone()
	route := changed.Models["claude"]
	route.Protocols = map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityText}}
	changed.Models["claude"] = route
	after, err := store.Commit(before, changed)
	if err != nil {
		t.Fatal(err)
	}
	newer := []byte("# newer operator content\n")
	for _, path := range []string{capabilityComponentPath(store), capabilityComponentPath(store) + ".bak"} {
		if err := os.WriteFile(path, newer, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err = store.RestoreSnapshot(before, after)
	for _, want := range []string{"restore route capability snapshot", "restore route capability backup"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("restore error = %v, want %q", err, want)
		}
	}
	for _, path := range []string{capabilityComponentPath(store), capabilityComponentPath(store) + ".bak"} {
		if got := readStoreFile(t, path); string(got) != string(newer) {
			t.Fatalf("restore replaced newer content at %s", filepath.Base(path))
		}
	}
}

// failCapabilityWrites makes every write or removal of path fail until the
// test ends, and returns the injected failure.
func failCapabilityWrites(t *testing.T, path string) error {
	t.Helper()
	failure := errors.New("capability component mutation failed")
	originalWrite := writeConfigurationFileIfUnchanged
	originalRemove := removeConfigurationFileIfUnchanged
	t.Cleanup(func() {
		writeConfigurationFileIfUnchanged = originalWrite
		removeConfigurationFileIfUnchanged = originalRemove
	})
	writeConfigurationFileIfUnchanged = func(target string, expected platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
		if target == path {
			return platform.FileSnapshot{}, failure
		}
		return originalWrite(target, expected, data, mode)
	}
	removeConfigurationFileIfUnchanged = func(target string, expected platform.FileSnapshot) (platform.FileSnapshot, error) {
		if target == path {
			return platform.FileSnapshot{}, failure
		}
		return originalRemove(target, expected)
	}
	return failure
}

// everyComponentStore returns a Store whose configuration holds all four
// optional components, each with a backup, and that configuration.
func everyComponentStore(t *testing.T) (Store, domain.Config) {
	t.Helper()
	cfg := convergenceConfig("current")
	account := cfg.Accounts["current"]
	account.Order = 1
	account.Endpoints.Anthropic = "https://current.test"
	cfg.Accounts["current"] = account
	cfg.Models["tool-search"] = domain.Model{
		Label: "Tool search", Account: "current", Family: "current-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolAnthropic: {domain.CapabilityToolSearch}},
	}
	forwardTo(t, &cfg, "http://127.0.0.1:8792/v1")
	zero := uint64(0)
	binding := cfg.Clients[ClientCodex]
	binding.RetryPolicy = &domain.RetryPolicy{RequestMaxRetries: &zero}
	cfg.Clients[ClientCodex] = binding
	store := NewStore(filepath.Join(t.TempDir(), "config.toml"), testAdmission())
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	cfg = relabeled(cfg, "Second generation")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return store, cfg
}

func relabeled(cfg domain.Config, label string) domain.Config {
	cfg = cfg.Clone()
	account := cfg.Accounts["current"]
	account.Label = label
	cfg.Accounts["current"] = account
	return cfg
}

// observeMutations records the file of every guarded write and removal until
// the test ends, refuses a mode other than the Store's own, and fails the
// mutation of the file named failing with the returned error.
func observeMutations(t *testing.T, failing string) (*[]string, error) {
	t.Helper()
	failure := errors.New("mutation failed")
	visited := &[]string{}
	originalWrite := writeConfigurationFileIfUnchanged
	originalRemove := removeConfigurationFileIfUnchanged
	t.Cleanup(func() {
		writeConfigurationFileIfUnchanged = originalWrite
		removeConfigurationFileIfUnchanged = originalRemove
	})
	writeConfigurationFileIfUnchanged = func(path string, expected platform.FileSnapshot, data []byte, mode os.FileMode) (platform.FileSnapshot, error) {
		*visited = append(*visited, filepath.Base(path))
		if mode != 0o600 {
			t.Errorf("%s written with mode %o", filepath.Base(path), mode)
		}
		if filepath.Base(path) == failing {
			return platform.FileSnapshot{}, failure
		}
		return originalWrite(path, expected, data, mode)
	}
	removeConfigurationFileIfUnchanged = func(path string, expected platform.FileSnapshot) (platform.FileSnapshot, error) {
		*visited = append(*visited, filepath.Base(path))
		if filepath.Base(path) == failing {
			return platform.FileSnapshot{}, failure
		}
		return originalRemove(path, expected)
	}
	return visited, failure
}

type componentStep struct{ file, failure string }

func stepFiles(steps []componentStep) []string {
	files := make([]string, 0, len(steps))
	for _, step := range steps {
		files = append(files, step.file)
	}
	return files
}

// A commit backs up, then writes, every file in one order, and each failure
// names its step.
func TestCommitVisitsEveryFileInOrder(t *testing.T) {
	steps := []componentStep{
		{"config.toml.bak", "back up current config"},
		{"config.forwarding.toml.bak", "back up client forwarding"},
		{"config.retries.toml.bak", "back up client retry policies"},
		{"config.catalog.toml.bak", "back up catalog presentation"},
		{"config.capabilities.toml.bak", "back up route capabilities"},
		{"config.toml", "write config"},
		{"config.forwarding.toml", "write client forwarding"},
		{"config.retries.toml", "write client retry policies"},
		{"config.catalog.toml", "write catalog presentation"},
		{"config.capabilities.toml", "write route capabilities"},
		{"config.toml.verified.json", "invalidate verified checkpoint"},
	}
	t.Run("order", func(t *testing.T) {
		store, cfg := everyComponentStore(t)
		before, err := store.CaptureSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		visited, _ := observeMutations(t, "")
		if _, err := store.Commit(before, relabeled(cfg, "Third generation")); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(*visited, stepFiles(steps)) {
			t.Fatalf("commit visited %q, want %q", *visited, stepFiles(steps))
		}
	})
	for _, step := range steps {
		t.Run(step.file, func(t *testing.T) {
			store, cfg := everyComponentStore(t)
			before, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			_, failure := observeMutations(t, step.file)
			want := step.failure + ": " + failure.Error()
			if _, err := store.Commit(before, relabeled(cfg, "Third generation")); err == nil || err.Error() != want {
				t.Fatalf("commit error = %v, want %q", err, want)
			}
			if after, err := store.CaptureSnapshot(); err != nil || !before.equal(after) {
				t.Fatalf("failed commit did not restore every owned file: %v", err)
			}
		})
	}
}

// Backup convergence checks every preimage in one order, then backs up every
// file in the order a commit does, and each failure names its step.
func TestVerifiedBackupConvergenceVisitsEveryFileInOrder(t *testing.T) {
	steps := []componentStep{
		{"config.toml.bak", "converge verified config backup"},
		{"config.forwarding.toml.bak", "converge verified forwarding backup"},
		{"config.retries.toml.bak", "converge verified retry policy backup"},
		{"config.catalog.toml.bak", "converge verified catalog backup"},
		{"config.capabilities.toml.bak", "converge verified route capability backup"},
	}
	t.Run("order", func(t *testing.T) {
		store, _ := everyComponentStore(t)
		expected, err := store.CaptureSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		visited, _ := observeMutations(t, "")
		if err := store.ConvergeVerifiedBackup(expected); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(*visited, stepFiles(steps)) {
			t.Fatalf("convergence visited %q, want %q", *visited, stepFiles(steps))
		}
	})
	for _, step := range steps {
		t.Run(step.file, func(t *testing.T) {
			store, _ := everyComponentStore(t)
			expected, err := store.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			_, failure := observeMutations(t, step.file)
			want := step.failure + ": " + failure.Error()
			if err := store.ConvergeVerifiedBackup(expected); err == nil || err.Error() != want {
				t.Fatalf("convergence error = %v, want %q", err, want)
			}
		})
	}
	t.Run("preimages", func(t *testing.T) {
		store, cfg := everyComponentStore(t)
		if err := store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}); err != nil {
			t.Fatal(err)
		}
		expected, err := store.CaptureSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		checks := []componentStep{
			{"config.toml", "config"},
			{"config.forwarding.toml", "client forwarding"}, {"config.forwarding.toml.bak", "client forwarding"},
			{"config.catalog.toml", "catalog presentation"}, {"config.catalog.toml.bak", "catalog presentation"},
			{"config.capabilities.toml", "route capability"}, {"config.capabilities.toml.bak", "route capability"},
			{"config.retries.toml", "client retry policy"}, {"config.retries.toml.bak", "client retry policy"},
			{"config.toml.verified.json", "verified checkpoint"},
		}
		directory := filepath.Dir(store.Path())
		original := map[string][]byte{}
		for _, check := range checks {
			path := filepath.Join(directory, check.file)
			original[check.file] = readStoreFile(t, path)
			if err := os.WriteFile(path, []byte("# newer operator content\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, check := range checks {
			want := check.failure + " preimage changed; refusing to converge backup"
			if err := store.ConvergeVerifiedBackup(expected); err == nil || err.Error() != want {
				t.Fatalf("with %s changed, convergence error = %v, want %q", check.file, err, want)
			}
			if err := os.WriteFile(filepath.Join(directory, check.file), original[check.file], 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// Restoration reaches every file in one order and names each conflict.
func TestRestoreSnapshotVisitsEveryFileInOrder(t *testing.T) {
	store, cfg := everyComponentStore(t)
	before, err := store.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.Commit(before, relabeled(cfg, "Third generation"))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, step := range []componentStep{
		{"config.toml", "restore config snapshot"},
		{"config.toml.bak", "restore config backup snapshot"},
		{"config.toml.verified.json", "restore verified checkpoint snapshot"},
		{"config.forwarding.toml", "restore client forwarding snapshot"},
		{"config.forwarding.toml.bak", "restore client forwarding backup snapshot"},
		{"config.catalog.toml", "restore catalog presentation snapshot"},
		{"config.catalog.toml.bak", "restore catalog presentation backup"},
		{"config.capabilities.toml", "restore route capability snapshot"},
		{"config.capabilities.toml.bak", "restore route capability backup"},
		{"config.retries.toml", "restore client retry policy snapshot"},
		{"config.retries.toml.bak", "restore client retry policy backup"},
	} {
		path := filepath.Join(filepath.Dir(store.Path()), step.file)
		if err := os.WriteFile(path, []byte("# newer operator content\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		want = append(want, fmt.Sprintf("%s: postimage changed for %s; refusing to overwrite newer state", step.failure, path))
	}
	if err := store.RestoreSnapshot(before, after); err == nil || err.Error() != strings.Join(want, "\n") {
		t.Fatalf("restore error = %v, want %q", err, strings.Join(want, "\n"))
	}
}

// A snapshot stops at an unreadable forwarding or catalog file, and reads both
// generations of the retry and capability components before it reports them.
// A read of the current generation alone reports every component.
func TestComponentReadFailuresKeepTheirOrder(t *testing.T) {
	store, cfg := everyComponentStore(t)
	directory := filepath.Dir(store.Path())
	block := func(file string) {
		t.Helper()
		path := filepath.Join(directory, file)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	assertFailure := func(err error, files ...string) {
		t.Helper()
		if err == nil {
			t.Fatalf("read succeeded, want failures for %q", files)
		}
		lines := strings.Split(err.Error(), "\n")
		if len(lines) != len(files) {
			t.Fatalf("read error = %q, want one line for each of %q", err, files)
		}
		for index, file := range files {
			if !strings.Contains(lines[index], filepath.Join(directory, file)+":") {
				t.Fatalf("read error line %d = %q, want %s", index, lines[index], file)
			}
		}
	}
	components := []string{"config.forwarding.toml", "config.retries.toml", "config.catalog.toml", "config.capabilities.toml"}
	for _, file := range components {
		block(file)
	}
	assertFailure(store.SaveVerifiedCheckpoint(t.Context(), cfg, []string{ClientCodex}), components...)
	for _, file := range components {
		block(file + ".bak")
	}
	for _, reported := range [][]string{
		{"config.forwarding.toml"},
		{"config.retries.toml", "config.retries.toml.bak"},
		{"config.catalog.toml"},
		{"config.capabilities.toml", "config.capabilities.toml.bak"},
	} {
		_, err := store.CaptureSnapshot()
		assertFailure(err, reported...)
		for _, path := range []string{reported[0], reported[0] + ".bak"} {
			if err := os.Remove(filepath.Join(directory, path)); err != nil {
				t.Fatal(err)
			}
		}
	}
}
