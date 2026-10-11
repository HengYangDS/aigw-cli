package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestVerificationProjectsAnotherModelWithoutChangingOwnedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("model:\n  temperature: 0.3\nagent:\n  reasoning_overrides:\n    another-model: none\nsecurity:\n  allow_lazy_installs: true\n  policy: strict\nupdates:\n  check: true\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(path + stateSuffix)
	if err != nil {
		t.Fatal(err)
	}
	desired.SelectedModel = "another-model"
	desired.Providers[0].Models = append(desired.Providers[0].Models, desired.SelectedModel)
	plan, err = Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	data, err := plan.PrepareVerification()
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	model := configMap(t, result, "model")
	security := configMap(t, result, "security")
	if model["default"] != "another-model" || model["temperature"] != 0.3 || security["allow_lazy_installs"] != false || security["policy"] != "strict" || configMap(t, result, "updates")["check"] != false {
		t.Fatalf("isolated native configuration = %#v", result)
	}
	for source, expected := range map[string][]byte{path: before, path + stateSuffix: state} {
		actual, err := os.ReadFile(source)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatal("verification changed the source configuration or ownership record")
		}
	}
}

func TestVerificationRejectsAmbiguousNativePolicy(t *testing.T) {
	for _, content := range []string{"security: [invalid]", "updates: &policy {}", "security: *missing", "model: [invalid]"} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			desired := testDesired()
			plan, err := Prepare(path, &desired)
			if err == nil {
				_, err = plan.PrepareVerification()
			}
			if err == nil {
				t.Fatal("ambiguous native policy was accepted")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != content {
				t.Fatal("failed verification preparation changed the source")
			}
		})
	}
}

func TestVerificationRequiresExistingNativeConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	desired := testDesired()
	plan, err := Prepare(path, &desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.PrepareVerification(); err == nil || !strings.Contains(err.Error(), "native configuration is missing") {
		t.Fatalf("missing native configuration was synthesized: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("verification wrote a missing source: %v", err)
	}
	if _, err := Prepare(t.TempDir(), &desired); err == nil {
		t.Fatal("directory was accepted as native configuration")
	}
}

func TestVerificationHomePreservesTheNativeRoot(t *testing.T) {
	for _, name := range []string{"ordinary-home", "named-profile"} {
		t.Run(name, func(t *testing.T) {
			named := name == "named-profile"
			root := t.TempDir()
			target := filepath.Join(root, "config.yaml")
			if named {
				target = filepath.Join(root, "profiles", "personal", "config.yaml")
				if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte("user-owned"), 0o600); err != nil {
					t.Fatal(err)
				}
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
			if filepath.Dir(home) != filepath.Join(root, "profiles") {
				t.Fatalf("verification profile %q lost the native root %q", home, root)
			}
			if err := os.WriteFile(filepath.Join(home, "owned"), []byte("temporary"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := cleanup(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(home); !os.IsNotExist(err) {
				t.Fatalf("owned profile remains: %v", err)
			}
			if named {
				if data, err := os.ReadFile(target); err != nil || string(data) != "user-owned" {
					t.Fatalf("native profile changed: %v", err)
				}
			} else if entries, err := os.ReadDir(filepath.Dir(home)); err != nil || len(entries) != 0 {
				t.Fatalf("native profile container was removed or holds residue: %v, %v", entries, err)
			}
		})
	}
}

func TestVerificationHomesShareTheNativeProfileContainer(t *testing.T) {
	for first, name := range []string{"creator-first", "borrower-first"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			profiles := filepath.Join(root, "profiles")
			var homes [2]string
			var cleanup [2]func() error
			for i := range homes {
				var err error
				homes[i], cleanup[i], err = CreateVerificationHome(filepath.Join(root, "config.yaml"))
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := cleanup[first](); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(homes[1-first]); err != nil {
				t.Fatalf("another active verification profile was removed: %v", err)
			}
			if err := cleanup[1-first](); err != nil {
				t.Fatal(err)
			}
			if entries, err := os.ReadDir(profiles); err != nil || len(entries) != 0 {
				t.Fatalf("native container was removed or holds verification residue: %v, %v", entries, err)
			}
		})
	}
}

func TestVerificationHomeCleanupAcceptsAWithdrawnParent(t *testing.T) {
	home, cleanup, err := CreateVerificationHome(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(home); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(home)); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("another operation already removed the empty profile parent: %v", err)
	}
}
