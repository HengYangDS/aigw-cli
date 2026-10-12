//go:build !windows

package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
				if err := os.WriteFile(target, []byte("# user-owned\nterminal: {backend: local}\n"), 0o600); err != nil {
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
				if data, err := os.ReadFile(target); err != nil || string(data) != "# user-owned\nterminal: {backend: local}\n" {
					t.Fatalf("native profile changed: %v", err)
				}
			} else if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatalf("native root holds verification residue: %v, %v", entries, err)
			}
		})
	}
}

func TestVerificationDependencySelectionRefusesScalarCoercion(t *testing.T) {
	for _, data := range []string{"plugins: {enabled: [123]}", "plugins: {disabled: [false]}", "plugins: {enabled: [null]}", "memory: {provider: true}", "plugins: [native]", "plugins: {enabled: native}", "plugins: ["} {
		t.Run(data, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "config.yaml")
			if err := os.WriteFile(target, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			home, cleanup, err := CreateVerificationHome(target)
			if err == nil {
				if err := cleanup(); err != nil {
					t.Error(err)
				}
				t.Fatalf("native invalid dependency selector was coerced into %s", home)
			}
			if actual, err := os.ReadFile(target); err != nil || string(actual) != data {
				t.Fatal("rejected dependency selection changed the operator config")
			}
		})
	}
}

func TestVerificationDependencyProfileContainsOnlySelectors(t *testing.T) {
	root := t.TempDir()
	plugins := filepath.Join(root, "plugins")
	for _, name := range []string{"plugins", "tools", "installs", "profiles/.deleted/retired"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(name)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	data := "plugins: {enabled: [native], disabled: [unused], token: private-fixture}\nmemory: {provider: memory, api_key: private-fixture}\nproviders: {personal: {api_key: private-fixture}}\n"
	target := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(target, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, selected := range map[string]string{
		"work":                 "plugins: {enabled: [native], disabled: [unused]}\n",
		"memory":               "memory: {provider: memory}\n",
		"retired":              "plugins: [invalid]\n",
		".work.staging-hidden": "plugins: [invalid]\n",
		"Bad Name":             "plugins: [invalid]\n",
	} {
		profile := filepath.Join(root, "profiles", name)
		if err := os.MkdirAll(profile, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(profile, "config.yaml"), []byte(selected), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(root, filepath.Join(root, "profiles", "default")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "installs", "metadata"), []byte("not an installation"), 0o600); err != nil {
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
	if err != nil || len(profiles) != 3 {
		t.Fatalf("isolated live dependency selectors: %v %v", profiles, err)
	}
	for _, profile := range profiles {
		selected, err := os.ReadFile(profile)
		if err != nil || bytes.Contains(selected, []byte("private-fixture")) {
			t.Fatal("dependency profile copied unrelated configuration or credentials")
		}
	}
	for borrowed, source := range map[string]string{
		filepath.Join(filepath.Dir(profiles[0]), "plugins"): plugins,
		filepath.Join(home, "tools"):                        filepath.Join(root, "tools"),
	} {
		actual, err := os.Stat(borrowed)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := os.Stat(source)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(actual, expected) {
			t.Fatalf("borrowed dependency identity changed: %s does not resolve to %s", borrowed, source)
		}
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(target); err != nil || string(actual) != data {
		t.Fatal("dependency selector cleanup changed native configuration")
	}
}

func TestVerificationDependencyBorrowingRefusesOccupiedPrivateState(t *testing.T) {
	for _, name := range []string{"install", "install-entry", "profiles", "profile"} {
		t.Run(name, func(t *testing.T) {
			root, private := t.TempDir(), t.TempDir()
			profile := filepath.Join(root, "profiles", "work")
			if err := os.MkdirAll(profile, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(profile, "config.yaml"), []byte("memory: {provider: native}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "environments"), []byte("committed dependency"), 0o600); err != nil {
				t.Fatal(err)
			}
			occupied := filepath.Join(private, name)
			switch name {
			case "install-entry":
				occupied = filepath.Join(private, "install", "environments")
			case "profile":
				occupied = filepath.Join(private, "profiles", "work")
			}
			if err := os.MkdirAll(filepath.Dir(occupied), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(occupied, []byte("owned state"), 0o600); err != nil {
				t.Fatal(err)
			}
			var err error
			if strings.HasPrefix(name, "install") {
				err = borrowVerificationInstall(root, filepath.Join(private, "install"))
			} else {
				err = borrowVerificationMembers(root, private)
			}
			if err == nil {
				t.Fatal("dependency borrowing accepted an occupied private destination")
			}
			if actual, err := os.ReadFile(occupied); err != nil || string(actual) != "owned state" {
				t.Fatal("dependency borrowing replaced existing private state")
			}
			if actual, err := os.ReadFile(filepath.Join(root, "environments")); err != nil || string(actual) != "committed dependency" {
				t.Fatal("refused dependency borrowing changed its source")
			}
		})
	}
}

func TestVerificationHomesHaveIndependentCleanup(t *testing.T) {
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
	for _, test := range []struct {
		name, path, want string
		file             bool
	}{
		{name: "tools", path: "tools", want: "dependency state"},
		{name: "install-index", path: "installs", want: "dependency selections", file: true},
		{name: "install-selection", path: "installs/selection", want: "dependency selection"},
		{name: "committed-environment", path: "installs/selection/environments", want: "committed Hermes dependency"},
		{name: "profile-index", path: "profiles", want: "dependency profiles", file: true},
		{name: "profile-retirement", path: "profiles/.deleted", want: "profile retirement"},
		{name: "live-profile", path: "profiles/work", want: "resolve Hermes dependency profile"},
		{name: "configuration", path: "config.yaml", want: "read Hermes dependency selection"},
		{name: "member-sources", path: "plugins", want: "read Hermes dependency members"},
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
			if test.name == "profile-retirement" {
				if err := os.Mkdir(filepath.Join(root, "profiles", "work"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if test.name == "member-sources" {
				if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("plugins: {enabled: [native]}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(unusable)
			if err != nil {
				t.Fatal(err)
			}
			requireVerificationHomeRefusal(t, filepath.Join(root, "config.yaml"), temporary, test.want)
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
			requireVerificationHomeRefusal(t, filepath.Join(root, "config.yaml"), temporary, want)
		})
	}
}

func requireVerificationHomeRefusal(t *testing.T, target, temporary, want string) {
	t.Helper()
	home, cleanup, err := CreateVerificationHome(target)
	if err == nil || !strings.Contains(err.Error(), want) || home != "" || cleanup != nil {
		t.Fatalf("unusable verification state was accepted: %q, %v", home, err)
	}
	if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
		t.Fatalf("failed preparation left temporary state: %v, %v", entries, err)
	}
}
