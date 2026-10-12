//go:build !windows

package configuration

import (
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
				t.Fatalf("owned verification home remains: %v", err)
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
				t.Fatalf("another active verification home was removed: %v", err)
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
		t.Fatalf("another operation already removed the empty verification parent: %v", err)
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

func TestVerificationHomeBorrowsCommittedDependenciesOnly(t *testing.T) {
	root := t.TempDir()
	installed := filepath.Join(root, "installs", "selection")
	for _, directory := range []string{filepath.Join(installed, "environments"), filepath.Join(installed, "pm-runtime"), filepath.Join(installed, "bootstrap"), filepath.Join(root, "tools")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"facts.json", ".install.lock"} {
		if err := os.WriteFile(filepath.Join(installed, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	home, cleanup, err := CreateVerificationHome(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	borrowed := map[string]string{"tools": filepath.Join(root, "tools")}
	for _, name := range []string{"facts.json", ".install.lock", "environments", "pm-runtime"} {
		borrowed[filepath.Join("installs", "selection", name)] = filepath.Join(installed, name)
	}
	for name, source := range borrowed {
		shared, sharedErr := os.Stat(filepath.Join(home, name))
		original, originalErr := os.Stat(source)
		if sharedErr != nil || originalErr != nil || !os.SameFile(shared, original) {
			t.Fatalf("verification home does not borrow the installed %s: %v %v", name, sharedErr, originalErr)
		}
	}
	if _, err := os.Lstat(filepath.Join(home, "installs", "selection", "bootstrap")); !os.IsNotExist(err) {
		t.Fatalf("verification home borrowed the operator's boot records: %v", err)
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
