//go:build !windows

package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
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

func TestVerificationHomeRefusesInvalidDependencySelectionWithoutResidue(t *testing.T) {
	for _, test := range []struct {
		name, config, diagnostic string
	}{
		{"undecodable document", "plugins: [", "cannot decode"},
		{"nonmapping section", "plugins: [member]\n", "independent mapping"},
		{"scalar membership", "plugins: {enabled: member}\n", "native lists"},
		{"nonstrings", "memory: {provider: 42}\n", "native string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, temporary := t.TempDir(), t.TempDir()
			t.Setenv("TMPDIR", temporary)
			target := filepath.Join(root, "config.yaml")
			original := []byte(test.config)
			if err := os.WriteFile(target, original, 0o600); err != nil {
				t.Fatal(err)
			}
			home, cleanup, err := CreateVerificationHome(target)
			if err == nil || !strings.Contains(err.Error(), test.diagnostic) || home != "" || cleanup != nil {
				t.Fatalf("invalid native selector was accepted: %s, %v", home, err)
			}
			if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
				t.Fatalf("refusal left runtime state: %v, %v", entries, err)
			}
			if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, original) {
				t.Fatalf("refusal changed operator configuration: %v", err)
			}
		})
	}
}

func TestVerificationHomeRequiresTemporaryStorageOutsideTheNativeRoot(t *testing.T) {
	root := t.TempDir()
	storage := filepath.Join(root, "temporary")
	if err := os.Mkdir(storage, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", storage)
	config := []byte("plugins: {enabled: [native-member]}\n")
	target := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(target, config, 0o600); err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := CreateVerificationHome(target)
	if err == nil || !strings.Contains(err.Error(), "outside its native data root") || home != "" || cleanup != nil {
		t.Fatalf("native-root scratch storage was accepted: %s, %v", home, err)
	}
	if entries, err := os.ReadDir(storage); err != nil || len(entries) != 0 {
		t.Fatalf("isolation refusal left runtime state: %v, %v", entries, err)
	}
	if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, config) {
		t.Fatalf("isolation refusal changed operator configuration: %v", err)
	}
}

func TestVerificationHomesWithoutSelectedDependenciesHaveIndependentCleanup(t *testing.T) {
	root, temporary := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", temporary)
	profile := filepath.Join(root, "profiles", "personal")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	config := []byte("plugins: null\nmemory: null\npersonal: preserved\n")
	target := filepath.Join(profile, "config.yaml")
	if err := os.WriteFile(target, config, 0o600); err != nil {
		t.Fatal(err)
	}
	var homes [2]string
	var cleanups [2]func() error
	for i := range homes {
		var err error
		homes[i], cleanups[i], err = CreateVerificationHome(target)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cleanups[i]() })
	}
	if homes[0] == homes[1] {
		t.Fatal("independent verification runs share mutable storage")
	}
	if err := cleanups[0](); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(homes[1]); err != nil {
		t.Fatalf("cleanup removed another active verification: %v", err)
	}
	if entries, err := os.ReadDir(homes[1]); err != nil || len(entries) != 0 {
		t.Fatalf("empty dependency selection retained operator state: %v, %v", entries, err)
	}
	if err := cleanups[1](); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
		t.Fatalf("independent cleanup left runtime state: %v, %v", entries, err)
	}
	if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, config) {
		t.Fatalf("verification changed native preferences: %v", err)
	}
}

func TestVerificationHomeRejectsDamagedDependencyStateBeforeUse(t *testing.T) {
	for _, name := range []string{"unresolved installation", "invalid profiles directory", "unreadable configuration"} {
		t.Run(name, func(t *testing.T) {
			root, temporary := t.TempDir(), t.TempDir()
			t.Setenv("TMPDIR", temporary)
			marker := filepath.Join(root, "operator-state")
			if err := os.WriteFile(marker, []byte("preserved\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "unresolved installation":
				if err := os.Mkdir(filepath.Join(root, "installs"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "absent"), filepath.Join(root, "installs", "selected")); err != nil {
					t.Fatal(err)
				}
			case "invalid profiles directory":
				if err := os.WriteFile(filepath.Join(root, "profiles"), []byte("preserved\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unreadable configuration":
				if err := os.Mkdir(filepath.Join(root, "config.yaml"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
			if err == nil || home != "" || cleanup != nil {
				t.Fatalf("damaged dependency state was borrowed: %s, %v", home, err)
			}
			if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
				t.Fatalf("preparation refusal left runtime state: %v, %v", entries, err)
			}
			if data, err := os.ReadFile(marker); err != nil || string(data) != "preserved\n" {
				t.Fatalf("preparation refusal changed operator state: %v", err)
			}
		})
	}
}
