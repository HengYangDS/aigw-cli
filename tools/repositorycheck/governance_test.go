package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGovernanceAcceptsRepository(t *testing.T) {
	root := repositoryRoot(t)
	if err := checkGovernance(root); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "governance"}); err != nil {
		t.Fatal(err)
	}
}

func TestGovernanceRejectsMissingFileAndCommand(t *testing.T) {
	root := governanceFixture(t)
	if err := checkGovernance(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(governanceFiles[0]))); err != nil {
		t.Fatal(err)
	}
	if err := checkGovernance(root); err == nil || !strings.Contains(err.Error(), "missing governance file") {
		t.Fatalf("missing file error = %v", err)
	}

	root = governanceFixture(t)
	path := filepath.Join(root, "README.md")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text = []byte(strings.Replace(string(text), localVerificationCommands[0]+"\n", "", 1))
	if err := os.WriteFile(path, text, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkGovernance(root); err == nil || !strings.Contains(err.Error(), "must list local verification command exactly") {
		t.Fatalf("missing command error = %v", err)
	}
}

func TestGovernanceRejectsInvalidOwnedSurfaces(t *testing.T) {
	tests := []struct {
		name string
		edit func(*testing.T, string)
		want string
	}{
		{
			name: "race bypass",
			edit: func(t *testing.T, root string) { appendFixture(t, root, "README.md", "go test -race ./...\n") },
			want: "bypasses the required coverage gate",
		},
		{
			name: "shell verification command",
			edit: func(t *testing.T, root string) {
				appendFixture(t, root, "README.md", `test -z "$(gofmt -l cmd internal tools)"`+"\n")
			},
			want: "shell-owned verification command",
		},
		{
			name: "shell proof gate",
			edit: func(t *testing.T, root string) {
				overwriteFixture(t, root, ".ethos/profile.toml", "[proof]\ncommand = [\"sh\", \"-c\", \"go test ./...\"]\n")
			},
			want: "shell-owned automation remains",
		},
		{
			name: "shell release command",
			edit: func(t *testing.T, root string) {
				overwriteFixture(t, root, ".ethos/release.toml", "local_installation_command = \"scripts/install.sh\"\n")
			},
			want: "shell-owned automation remains",
		},
		{
			name: "tracked hook runtime",
			edit: func(t *testing.T, root string) {
				overwriteFixture(t, root, ".githooks/pre-commit", "#!/bin/sh\nethos hook admit pre-tool\n")
			},
			want: "tracked hook runtime remains",
		},
		{
			name: "product surface",
			edit: func(t *testing.T, root string) { overwriteFixture(t, root, "LICENSE", "invalid\n") },
			want: "product surface",
		},
		{
			name: "non English text",
			edit: func(t *testing.T, root string) {
				appendFixture(t, root, "README.md", "\u4e2d\u6587\n")
				gitRepository(t, root, "add", "README.md")
			},
			want: "English-only",
		},
		{
			name: "credential literal",
			edit: func(t *testing.T, root string) {
				overwriteFixture(t, root, "internal/leak.go", "package internal\nconst token = \"sk-abcdefghijklmnopqrstuvwxyz012345\"\n")
				gitRepository(t, root, "add", "internal/leak.go")
			},
			want: "credential-shaped literal",
		},
		{
			name: "retired docs",
			edit: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "docs", "history"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "retired documentary path remains",
		},
		{
			name: "missing ignore",
			edit: func(t *testing.T, root string) { overwriteFixture(t, root, ".gitignore", "") },
			want: ".gitignore must exclude",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := governanceFixture(t)
			test.edit(t, root)
			if err := checkGovernance(root); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGovernanceReportsReadErrors(t *testing.T) {
	root := governanceFixture(t)
	if _, err := readText(root, "missing"); err == nil || !strings.Contains(err.Error(), "read missing") {
		t.Fatalf("read error = %v", err)
	}
}

func TestGovernanceHelpersCoverReadAndFilesystemErrorBoundaries(t *testing.T) {
	root := governanceFixture(t)
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "README.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkGovernance(root); err == nil {
		t.Fatal("directory governance document accepted")
	}

	root = governanceFixture(t)
	if err := os.Remove(filepath.Join(root, ".ethos", "profile.toml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".ethos", "profile.toml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := checkGovernance(root); err == nil {
		t.Fatal("unreadable profile accepted")
	}

	if !pathsExist(root, []string{".ethos/profile.toml"}) {
		t.Fatal("existing path was not detected")
	}
	if pathsExist(root, []string{"missing"}) {
		t.Fatal("missing path was reported present")
	}

	root = governanceFixture(t)
	retired := filepath.Join(root, "docs", "history")
	if err := os.MkdirAll(retired, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(retired, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(retired, 0o700) })
	if err := checkGovernance(root); err == nil {
		t.Fatal("unreadable retired documentary path accepted")
	}

	root = governanceFixture(t)
	if err := os.Remove(filepath.Join(root, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	if err := checkGovernance(root); err == nil || !strings.Contains(err.Error(), "read .gitignore") {
		t.Fatalf("missing ignore error = %v", err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func governanceFixture(t *testing.T) string {
	t.Helper()
	root := productSurfaceRepository(t)
	for _, relative := range governanceFiles {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte("fixture\n"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	commands := strings.Join(localVerificationCommands, "\n") + "\n"
	for _, relative := range []string{"README.md", "CONTRIBUTING.md", "AGENTS.md"} {
		path := filepath.Join(root, relative)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, []byte(commands)...), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".serena/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func appendFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func overwriteFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
