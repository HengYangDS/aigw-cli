package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/config"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/discovery"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/manifest"
)

func TestAdvancedThresholdMissingExecutables(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	app := dailyCoverageApp(t, dailyCoverageConfig())
	cmd := newAdapterDiscoverCommand(app)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := app.Out.(*bytes.Buffer)
	if strings.Count(output.String(), "Not found") != len(domain.AdmittedClientIDs()) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestAdvancedThresholdConfigLoadFailures(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(*App) *cobra.Command
		args []string
	}{
		{name: "adapter enable", cmd: func(app *App) *cobra.Command { return newAdapterEnableCommand(app) }, args: []string{"claude", "--executable", "/claude"}},
		{name: "adapter auth", cmd: func(app *App) *cobra.Command { return newAdapterAuthCommand(app) }, args: []string{"codex"}},
		{name: "adapter disable", cmd: func(app *App) *cobra.Command { return newAdapterDisableCommand(app) }, args: []string{"codex"}},
		{name: "profile rename", cmd: func(app *App) *cobra.Command { return newProfileRenameCommand(app) }, args: []string{"old", "new"}},
		{name: "sync", cmd: func(app *App) *cobra.Command { return newSyncCommand(app) }},
		{name: "rollback", cmd: func(app *App) *cobra.Command { return newRollbackCommand(app) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := dailyCoverageApp(t, dailyCoverageConfig())
			app.Config = config.NewStore(t.TempDir())
			cmd := test.cmd(app)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(test.args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected config load error")
			}
		})
	}
}

func TestAdvancedThresholdTargetAndNamingBranches(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "codex")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	discovered := discovery.Result{Surfaces: []discovery.Surface{
		{ID: discovery.SurfaceCodexCLIStandalone, Executable: executable},
		{ID: "future-surface", ConfigPath: configPath},
	}}
	if err := validateExplicitCodexTarget(discovered, executable); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("executable error = %v", err)
	}
	if err := validateExplicitCodexTarget(discovered, configPath); err == nil || !strings.Contains(err.Error(), "future-surface") {
		t.Fatalf("surface error = %v", err)
	}
	if names := importedAccountNames(manifest.Manifest{Profiles: map[string]domain.Profile{"implicit": {}}}); len(names) != 1 || names[0] != "implicit" {
		t.Fatalf("account names = %#v", names)
	}
	if got := title(""); got != "" {
		t.Fatalf("empty title = %q", got)
	}
}

func TestAdvancedThresholdRouteAndReconciliationErrors(t *testing.T) {
	invalidStoreApp := dailyCoverageApp(t, dailyCoverageConfig())
	invalidStoreApp.Config = config.NewStore(t.TempDir())
	if err := runRouteList(invalidStoreApp); err == nil {
		t.Fatal("expected route-list config error")
	}

	emptyApp := dailyCoverageApp(t, dailyCoverageConfig())
	emptyApp.Config = config.NewStore(filepath.Join(t.TempDir(), "missing.toml"))
	if err := runRouteList(emptyApp); err == nil || !strings.Contains(strings.ToLower(err.Error()), "not configured") {
		t.Fatalf("empty route-list error = %v", err)
	}

	cfg := dailyCoverageConfig()
	cfg.Adapters[domain.ClientCodex] = domain.AdapterConfig{Enabled: true, Targets: []string{filepath.Join(t.TempDir(), "config.toml")}}
	if _, err := planCodexReconciliation(&App{}, cfg, cfg); err == nil {
		t.Fatal("expected reconciliation planning error")
	}
	if err := reconcileCodexProjection(context.Background(), &App{}, cfg, cfg); err == nil {
		t.Fatal("expected reconciliation error")
	}

	broken := dailyCoverageConfig()
	profile := broken.Profiles["one"]
	profile.Account = "missing"
	broken.Profiles["alias"] = profile
	if _, _, err := accountForInput(broken, "alias"); err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Fatalf("account error = %v", err)
	}
}
