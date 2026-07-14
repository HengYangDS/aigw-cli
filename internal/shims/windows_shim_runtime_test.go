//go:build windows

package shims_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

func TestManagerCreatesWindowsCommandShimThatCanRunAIGW(t *testing.T) {
	dir := t.TempDir()
	manager := shims.Manager{GOOS: "windows", BinDir: dir, AIGWExecutable: os.Args[0]}
	path, err := manager.EnableClaude()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".cmd" {
		t.Fatalf("shim path = %q, want .cmd", path)
	}
	output, err := exec.Command(path, "-test.run=^$").CombinedOutput()
	if err != nil {
		t.Fatalf("run Windows Claude shim: %v: %s", err, output)
	}
	if !strings.Contains(string(output), "PASS") {
		t.Fatalf("Windows Claude shim output = %q", output)
	}
}
