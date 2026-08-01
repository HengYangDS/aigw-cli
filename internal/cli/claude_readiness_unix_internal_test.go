//go:build !windows

package cli

import (
	"aigw-cli/internal/cli/readiness"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
)

func readyClaudeLauncher(t *testing.T) claude.Launcher {
	t.Helper()
	executable, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("resolve Go executable for Claude launcher fixture: %v", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		t.Fatal(err)
	}
	manager := claude.Launcher{
		GOOS:           runtime.GOOS,
		Home:           t.TempDir(),
		BinDir:         filepath.Join(t.TempDir(), "bin"),
		AIGWExecutable: executable,
	}
	if _, err := manager.EnableClaude(); err != nil {
		t.Fatal(err)
	}
	return manager
}

func unreadableClaudeLauncher(t *testing.T) claude.Launcher {
	t.Helper()
	manager := claude.Launcher{GOOS: runtime.GOOS, BinDir: t.TempDir()}
	if err := os.Mkdir(filepath.Join(manager.BinDir, "claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	return manager
}

func missingClaudeLauncher(t *testing.T) claude.Launcher {
	t.Helper()
	return claude.Launcher{GOOS: runtime.GOOS, BinDir: t.TempDir()}
}

func assertClaudeActivationBehavior(t *testing.T, app *App, cfg configuration.Config, runtimeConfig configuration.Runtime) {
	t.Helper()
	app.ClaudeLauncher = readyClaudeLauncher(t)
	activation := filepath.Join(app.ClaudeLauncher.Home, ".profile")
	if err := os.Remove(activation); err != nil {
		t.Fatal(err)
	}
	if ready, issue := readiness.AdapterRouteReady(app.invocationContext(), cfg, configuration.ClientClaude, runtimeConfig); ready || !strings.Contains(issue, "activation is missing") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	if err := os.Mkdir(activation, 0o700); err != nil {
		t.Fatal(err)
	}
	if ready, issue := readiness.AdapterRouteReady(app.invocationContext(), cfg, configuration.ClientClaude, runtimeConfig); ready || !strings.Contains(issue, "Cannot read Claude PATH") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
}
