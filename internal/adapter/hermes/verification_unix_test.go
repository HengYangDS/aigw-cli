//go:build !windows

package hermes

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Ported from the maintenance branch (974125a1): an installation that has not
// committed its dependency facts is not provisioned, while the native
// dependency selectors are retained without operator secrets or preferences.
func TestVerificationHomeRetainsNativeSelectorsWithoutProvisioning(t *testing.T) {
	root, temporary := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", temporary)
	installed := filepath.Join(root, "installs", "pending")
	if err := os.MkdirAll(installed, 0o700); err != nil {
		t.Fatal(err)
	}
	config := []byte("plugins: {enabled: [], disabled: [unused]}\nmemory: {provider: local, api_key: private-fixture}\nmodel: {default: operator-model}\n")
	target := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(target, config, 0o600); err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := CreateVerificationHome(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	profiles, err := filepath.Glob(filepath.Join(home, "profiles", "*", "config.yaml"))
	if err != nil || len(profiles) != 1 {
		t.Fatalf("dependency profile = %v, %v", profiles, err)
	}
	data, err := os.ReadFile(profiles[0])
	if err != nil || !bytes.Contains(data, []byte("provider: local")) || !bytes.Contains(data, []byte("unused")) || bytes.Contains(data, []byte("private-fixture")) || bytes.Contains(data, []byte("operator-model")) {
		t.Fatalf("dependency-only selection was not preserved: %s, %v", data, err)
	}
	if entries, err := os.ReadDir(filepath.Join(home, "installs", "pending")); err != nil || len(entries) != 0 {
		t.Fatalf("uncommitted installation was provisioned: %v, %v", entries, err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
		t.Fatalf("verification left runtime state: %v, %v", entries, err)
	}
	if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, config) {
		t.Fatalf("verification changed operator configuration: %v", err)
	}
}

// Ported from the maintenance branch (974125a1): null plugin and memory
// sections select no dependencies, so the verification home holds nothing.
func TestVerificationHomeWithNullSelectorsCreatesNoPrivateProfile(t *testing.T) {
	root, temporary := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", temporary)
	config := []byte("plugins: null\nmemory: null\npersonal: preserved\n")
	target := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(target, config, 0o600); err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := CreateVerificationHome(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
		t.Fatalf("an empty dependency selection retained operator state: %v, %v", entries, err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, config) {
		t.Fatalf("verification changed native preferences: %v", err)
	}
}
