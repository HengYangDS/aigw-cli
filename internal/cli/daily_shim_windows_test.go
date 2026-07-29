//go:build windows

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

func readyDailyShim(t *testing.T) shims.Manager {
	t.Helper()
	executable := filepath.Join(t.TempDir(), "aigw.exe")
	if err := os.WriteFile(executable, []byte("windows executable fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := shims.Manager{
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

func unreadableDailyShim(t *testing.T) shims.Manager {
	t.Helper()
	manager := shims.Manager{GOOS: runtime.GOOS, BinDir: t.TempDir()}
	if err := os.Mkdir(filepath.Join(manager.BinDir, "claude.cmd"), 0o700); err != nil {
		t.Fatal(err)
	}
	return manager
}

func missingDailyShim(t *testing.T) shims.Manager {
	t.Helper()
	return shims.Manager{GOOS: runtime.GOOS, BinDir: t.TempDir()}
}

func assertDailyClaudeActivationBehavior(t *testing.T, app *App, cfg domain.Config, runtimeConfig domain.Runtime) {
	t.Helper()
	app.Shims = readyDailyShim(t)
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientClaude, runtimeConfig); !ready || issue != "" {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	if active, err := app.Shims.ClaudeActivationReady(); err != nil || !active {
		t.Fatalf("Windows Claude activation ready=%v error=%v", active, err)
	}
	assertNoUnixActivationProfile(t, app.Shims.Home)
}

func assertDoctorClaudeActivationBehavior(t *testing.T, app *App) {
	t.Helper()
	app.Shims = readyDailyShim(t)
	check := requireDoctorCheck(t, executeDoctorJSON(t, app), "path:claude")
	if !check.OK || check.Detail != "AIGW-managed shell PATH activation" || check.Fix != "" {
		t.Fatalf("Windows Claude activation check = %#v", check)
	}
	assertNoUnixActivationProfile(t, app.Shims.Home)
}

func assertNoUnixActivationProfile(t *testing.T, home string) {
	t.Helper()
	for _, name := range []string{".profile", ".zshrc", ".bash_profile", ".bashrc"} {
		if _, err := os.Stat(filepath.Join(home, name)); !os.IsNotExist(err) {
			t.Fatalf("Windows shim wrote Unix activation profile %s: %v", name, err)
		}
	}
}
