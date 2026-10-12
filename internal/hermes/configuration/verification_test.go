package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestVerificationProjectsAnotherModelWithoutChangingOwnedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("model:\n  temperature: 0.3\nagent:\n  reasoning_overrides:\n    another-model: none\nsecurity:\n  allow_lazy_installs: true\n  policy: strict\nupdates:\n  check: true\ncli:\n  expose_on_path: true\n")
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
	if model["default"] != "another-model" || model["temperature"] != 0.3 || security["allow_lazy_installs"] != false || security["policy"] != "strict" || configMap(t, result, "updates")["check"] != false || configMap(t, result, "cli")["expose_on_path"] != false {
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
	if runtime.GOOS == "windows" {
		if _, _, err := CreateVerificationHome(filepath.Join(t.TempDir(), "config.yaml")); err == nil {
			t.Fatal("unisolated Windows verification was permitted")
		}
		return
	}
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
			if relative, err := filepath.Rel(root, home); err != nil || !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
				t.Fatalf("verification home %q shares the native root %q", home, root)
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
			} else if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatalf("native root holds verification residue: %v, %v", entries, err)
			}
		})
	}
}

func TestVerificationHomesHaveIndependentCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // Windows refusal is covered by TestVerificationHomePreservesTheNativeRoot.
	}
	for first, name := range []string{"creator-first", "borrower-first"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
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
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatalf("native root holds verification residue: %v, %v", entries, err)
			}
		})
	}
}

func TestVerificationHomeCleanupAcceptsAWithdrawnParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // Windows refusal is covered by TestVerificationHomePreservesTheNativeRoot.
	}
	t.Setenv("TMPDIR", t.TempDir())
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

func TestVerificationHomeBorrowsTheOriginalSelectionLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // Windows verification refuses before any dependency borrowing.
	}
	root := t.TempDir()
	installed := filepath.Join(root, "installs", "selection")
	if err := os.MkdirAll(installed, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(installed, ".install.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	before, err := os.Stat(lock)
	if err != nil {
		t.Fatal(err)
	}
	borrowed, err := os.Stat(filepath.Join(home, "installs", "selection", ".install.lock"))
	if err != nil || !os.SameFile(before, borrowed) {
		t.Fatalf("shared dependency selection has a different lock: %v", err)
	}
}

func TestVerificationHomeRefusesUnprotectedDependencySelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // Windows verification refuses before any dependency borrowing.
	}
	root := t.TempDir()
	installed := filepath.Join(root, "installs", "selection")
	if err := os.MkdirAll(installed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "facts.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
	if cleanup != nil {
		_ = cleanup()
	}
	if err == nil || home != "" || !strings.Contains(err.Error(), "selection lock") {
		t.Fatalf("unprotected shared selection was borrowed: %q, %v", home, err)
	}
}

func TestVerificationHomeRejectsPhysicalNativeRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // Windows verification refuses before creating a home.
	}
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "temporary")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", alias)
	home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
	if cleanup != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			t.Fatal(cleanupErr)
		}
	}
	if err == nil || home != "" || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("verification accepted temporary storage inside its physical native root: %q, %v", home, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("rejected verification left native residue: %v, %v", entries, err)
	}
}

func TestVerificationHomeRefusesUnusableNativeDependencies(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // Windows verification refuses before borrowing dependencies.
	}
	for _, test := range []struct {
		name, path, want string
		file             bool
	}{
		{name: "tools", path: "tools", want: "dependency state"},
		{name: "install-index", path: "installs", want: "dependency selections", file: true},
		{name: "install-selection", path: "installs/selection", want: "dependency selection"},
		{name: "committed-environment", path: "installs/selection/environments", want: "committed Hermes dependency"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, temporary := t.TempDir(), t.TempDir()
			t.Setenv("TMPDIR", temporary)
			t.Setenv("HOME", t.TempDir())
			unusable := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(unusable), 0o700); err != nil {
				t.Fatal(err)
			}
			var err error
			if test.file {
				err = os.WriteFile(unusable, []byte("native-state"), 0o600)
			} else {
				err = os.Symlink(unusable, unusable)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(unusable)
			if err != nil {
				t.Fatal(err)
			}
			home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
			if err == nil || !strings.Contains(err.Error(), test.want) || home != "" || cleanup != nil {
				t.Fatalf("unusable native state was borrowed: %q, %v", home, err)
			}
			if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
				t.Fatalf("failed preparation left a verification home: %v, %v", entries, err)
			}
			after, err := os.Lstat(unusable)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("refusal replaced native dependency state: %v", err)
			}
			if test.file {
				if data, err := os.ReadFile(unusable); err != nil || string(data) != "native-state" {
					t.Fatalf("refusal changed native dependency state: %v", err)
				}
			} else if target, err := os.Readlink(unusable); err != nil || target != unusable {
				t.Fatalf("refusal changed native dependency ownership: %v", err)
			}
		})
	}
}

func TestVerificationHomeRefusesUnresolvablePlacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		return // Windows verification refuses before creating a home.
	}
	for _, name := range []string{"temporary-storage", "user-home", "native-root"} {
		t.Run(name, func(t *testing.T) {
			root, temporary := t.TempDir(), t.TempDir()
			t.Setenv("TMPDIR", temporary)
			t.Setenv("HOME", t.TempDir())
			want := ""
			switch name {
			case "temporary-storage":
				t.Setenv("TMPDIR", filepath.Join(temporary, "absent"))
				want = "cannot create"
			case "user-home":
				t.Setenv("HOME", "")
				want = "native Hermes home"
			case "native-root":
				root = filepath.Join(root, "unresolvable")
				if err := os.Symlink(root, root); err != nil {
					t.Fatal(err)
				}
				want = "native Hermes data root"
			}
			home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
			if err == nil || !strings.Contains(err.Error(), want) || home != "" || cleanup != nil {
				t.Fatalf("unresolvable placement was accepted: %q, %v", home, err)
			}
			if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
				t.Fatalf("failed preparation left temporary state: %v, %v", entries, err)
			}
		})
	}
}
