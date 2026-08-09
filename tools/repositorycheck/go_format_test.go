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
	if err := checkGoFormat(root); err == nil || !strings.Contains(err.Error(), "cmd/tool/main.go") {
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
