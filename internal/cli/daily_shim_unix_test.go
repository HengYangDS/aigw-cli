//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/shims"
)

func readyDailyShim(t *testing.T) shims.Manager {
	t.Helper()
	executable, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("resolve Go executable for Claude shim fixture: %v", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
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
	if err := os.Mkdir(filepath.Join(manager.BinDir, "claude"), 0o700); err != nil {
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
	activation := filepath.Join(app.Shims.Home, ".profile")
	if err := os.Remove(activation); err != nil {
		t.Fatal(err)
	}
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientClaude, runtimeConfig); ready || !strings.Contains(issue, "activation is missing") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
	if err := os.Mkdir(activation, 0o700); err != nil {
		t.Fatal(err)
	}
	if ready, issue := adapterRouteReady(app, cfg, domain.ClientClaude, runtimeConfig); ready || !strings.Contains(issue, "Cannot read Claude PATH") {
		t.Fatalf("ready=%v issue=%q", ready, issue)
	}
}

func assertDoctorClaudeActivationBehavior(t *testing.T, app *App) {
	t.Helper()
	app.Shims = readyDailyShim(t)
	activation := filepath.Join(app.Shims.Home, ".profile")
	if err := os.Remove(activation); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(activation, 0o700); err != nil {
		t.Fatal(err)
	}
	check := requireDoctorCheck(t, executeDoctorJSON(t, app), "path:claude")
	if check.OK || !strings.Contains(check.Detail, "inspect Claude shell activation") || check.Fix != "run `aigw repair`" {
		t.Fatalf("Claude activation directory-read check = %#v", check)
	}
}
