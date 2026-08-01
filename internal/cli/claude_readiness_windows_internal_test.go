//go:build windows

package cli

import (
	"aigw-cli/internal/cli/readiness"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
)

func readyClaudeLauncher(t *testing.T) claude.Launcher {
	t.Helper()
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("windows executable fixture"), 0o600); err != nil {
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
	if err := os.Mkdir(filepath.Join(manager.BinDir, "claude.cmd"), 0o700); err != nil {
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
	if ready, issue := readiness.AdapterRouteReady(app.invocationContext(), cfg, configuration.ClientClaude, runtimeConfig); !ready || issue != "" {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	if active, err := app.ClaudeLauncher.ClaudeActivationReady(); err != nil || !active {
		t.Fatalf("Windows Claude activation ready=%v error=%v", active, err)
	}
	assertNoUnixActivationProfile(t, app.ClaudeLauncher.Home)
}
