package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoFormatAcceptsFormattedSourceAndRejectsDrift(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cmd", "tool", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkGoFormat(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package main\nfunc main(){println(\"drift\")}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join("cmd", "tool", "main.go")
	if err := checkGoFormat(root); err == nil || !strings.Contains(err.Error(), wantPath) {
		t.Fatalf("format drift error = %v", err)
	}
}

func TestGoFormatAcceptsRepositoryWithoutGoRoots(t *testing.T) {
	if err := checkGoFormat(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestGoFormatReportsParserFailure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "broken", "broken.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package broken\nfunc (\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkGoFormat(root); err == nil || !strings.Contains(err.Error(), "check Go formatting") {
		t.Fatalf("gofmt parser error = %v", err)
	}
}

func TestGoFormatIgnoresNonDirectoryRootsAndReportsStatFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cmd"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkGoFormat(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "cmd")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("cmd", filepath.Join(root, "cmd")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if err := checkGoFormat(root); err == nil || !strings.Contains(err.Error(), "inspect Go source root") {
		t.Fatalf("stat error = %v", err)
	}
}
