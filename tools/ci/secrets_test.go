package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestSecretChecksMeasureCurrentAuthoredFiles(t *testing.T) {
	repository := repositoryRoot(t)
	caller := t.TempDir()
	root := filepath.Join(caller, "checkout with spaces")
	write := func(path, content string) {
		t.Helper()
		path = filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{".gitignore", ".config/checks/secrets/policy.toml", ".config/checks/architecture/policy.toml"} {
		write(path, string(readFile(t, filepath.Join(repository, filepath.FromSlash(path)))))
	}
	git := func(args ...string) {
		t.Helper()
		call := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := call.CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v\n%s", err, output)
		}
	}
	git("init", "--quiet")
	secret := strings.Join([]string{"aB9x", "V2mN", "r4Qz", "W7kL", "p5S8", "j0C3", "D6fG", "h1T4"}, "")
	content := "api_key = " + strconv.Quote(secret) + "\n"
	write("build/signature.log", content)
	write("tracked.txt", "safe\n")
	git("add", "tracked.txt")
	git("-c", "core.hooksPath=", "-c", "commit.gpgsign=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "fixture")
	write("deleted.txt", content)
	git("add", "deleted.txt")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(caller)
	for _, test := range []struct {
		name, root, path string
		stage            bool
	}{
		{"absolute root", root, "", false},
		{"relative root", filepath.Base(root), "", false},
		{"unstaged current bytes", root, "tracked.txt", false},
		{"staged current bytes", root, "staged.txt", true},
		{"untracked current bytes", root, "new.txt", false},
		{"tracked ignored source", root, "build/tracked.txt", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.path != "" {
				write(test.path, content)
				if test.stage {
					git("add", "--force", test.path)
				}
				t.Cleanup(func() { write(test.path, "safe\n") })
			}
			var output []byte
			var scanRoot string
			err := run([]string{"check-secrets", test.root}, &bytes.Buffer{}, func(call command) error {
				scanRoot = call.Dir
				want := []string{"dir", "--config", ".config/checks/secrets/policy.toml", "--redact", "--no-banner", "--log-level", "warn", "."}
				if call.Name != "gitleaks" || !slices.Equal(call.Args, want) {
					t.Fatalf("native secret scanner contract: %#v", call)
				}
				call.Args = append(call.Args, "--report-format", "json", "--report-path", "-")
				var err error
				output, err = systemOutputRunner(call)
				return err
			})
			if test.path == "" && (err != nil || strings.TrimSpace(string(output)) != "[]") {
				t.Fatalf("clean source and path-specific policy: %v\n%s", err, output)
			}
			if test.path != "" {
				if err == nil || !bytes.Contains(output, []byte(test.path)) || bytes.Contains(output, []byte(secret)) {
					t.Fatalf("source finding must fail with relative path and redaction: %v\n%s", err, output)
				}
				if got := string(readFile(t, filepath.Join(root, filepath.FromSlash(test.path)))); got != content {
					t.Fatal("secret scan changed source")
				}
			}
			if scanRoot == "" || scanRoot == root {
				t.Fatal("secret scan did not use an isolated authored-file projection")
			}
			if _, err := os.Stat(scanRoot); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("scan scratch remains: %v", err)
			}
		})
	}
}

func TestSecretScanStaysOutsideGoPackageDiscovery(t *testing.T) {
	module := t.TempDir()
	root := filepath.Join(module, "source")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(module, "go.mod"):   "module fixture\n",
		filepath.Join(root, "product.go"): "package product\n",
		filepath.Join(root, ".gitignore"): "/build/\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	failure := errors.New("scanner failed")
	for _, scannerErr := range []error{nil, failure} {
		var scanRoot string
		err := checkSecrets(root, func(call command) error {
			scanRoot = call.Dir
			if got := string(readFile(t, filepath.Join(scanRoot, "product.go"))); got != "package product\n" {
				t.Fatalf("scanner did not receive current source: %q", got)
			}
			list := exec.Command("go", "list", "./...")
			list.Dir = module
			output, err := list.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != "fixture/source" {
				t.Errorf("temporary scanner input became product source: %s, %v", output, err)
			}
			return scannerErr
		})
		if !errors.Is(err, scannerErr) {
			t.Errorf("scanner error = %v, want %v", err, scannerErr)
		}
		if _, err := os.Stat(scanRoot); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("scanner retained its projection: %v", err)
		}
	}
}

func TestSecretChecksPreserveNativeSymlinkScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires privileges not granted to ordinary Windows users")
	}
	root, outside := t.TempDir(), filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside repository"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	link := filepath.Join(root, "linked.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("scanner interrupted")
	var scanRoot string
	err := run([]string{"check-secrets", root}, &bytes.Buffer{}, func(call command) error {
		scanRoot = call.Dir
		entries, err := os.ReadDir(call.Dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("symlink target entered scan: %v %v", entries, err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("scanner failure was lost: %v", err)
	}
	if _, err := os.Stat(scanRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed scan retained scratch: %v", err)
	}
	if target, err := os.Readlink(link); err != nil || target != outside || string(readFile(t, outside)) != "outside repository" {
		t.Fatalf("scan changed the symlink or its target: %q %v", target, err)
	}
}

func TestSecretChecksFailWithoutSourceOrNativePolicy(t *testing.T) {
	root := t.TempDir()
	if err := run([]string{"check-secrets", root}, &bytes.Buffer{}, systemRunner); err == nil {
		t.Fatal("nonrepository was accepted")
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	if err := run([]string{"check-secrets", root}, &bytes.Buffer{}, systemRunner); err == nil {
		t.Fatal("empty repository was accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("safe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var scanRoot string
	err := run([]string{"check-secrets", root}, &bytes.Buffer{}, func(call command) error {
		scanRoot = call.Dir
		_, err := systemOutputRunner(call)
		return err
	})
	if err == nil {
		t.Fatal("missing policy was accepted")
	}
	if _, err := os.Stat(scanRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing-policy failure retained scratch: %v", err)
	}
}
