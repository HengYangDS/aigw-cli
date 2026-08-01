package synchronization

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"aigw-cli/internal/codex"
	configuration "aigw-cli/internal/configuration"
)

const codexAuthenticationTimeout = 20 * time.Second

// BindAuthentication updates Codex native authentication for all configured
// targets without starting or restarting a Codex client.
func (s Synchronizer) BindAuthentication(ctx context.Context, cfg configuration.Config) error {
	adapter := cfg.Adapters[configuration.ClientCodex]
	if !adapter.Enabled {
		return nil
	}
	return s.BindAuthenticationTargets(ctx, cfg, adapter.Targets)
}

// BindAuthenticationTargets binds one explicit set of Codex homes.
func (s Synchronizer) BindAuthenticationTargets(ctx context.Context, cfg configuration.Config, targets []string) error {
	adapter := cfg.Adapters[configuration.ClientCodex]
	if !adapter.Enabled {
		return fmt.Errorf("Codex authentication requires an enabled adapter")
	}
	if adapter.Executable == "" || s.Runner == nil {
		return fmt.Errorf("Codex authentication requires an enabled adapter executable")
	}
	runtime, _, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return err
	}
	if s.Secrets == nil {
		return fmt.Errorf("Token for the Codex route is unavailable: secret store is unavailable")
	}
	token, err := s.Secrets.Get(runtime.AccountID)
	if err != nil {
		return fmt.Errorf("Token for the Codex route is unavailable: %w", err)
	}
	for _, target := range targets {
		plan, err := codex.LoginPlan(adapter.Executable, filepath.Dir(target), token)
		if err != nil {
			return err
		}
		targetCtx, cancel := context.WithTimeout(ctx, codexAuthenticationTimeout)
		err = s.Runner.Run(targetCtx, plan)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

// RouteAccount resolves the account used by the active Codex route.
func RouteAccount(cfg configuration.Config) (string, bool) {
	if !cfg.Adapters[configuration.ClientCodex].Enabled {
		return "", false
	}
	runtime, _, err := cfg.ResolveRuntime(configuration.ClientCodex, "")
	if err != nil {
		return "", false
	}
	return runtime.AccountID, runtime.AccountID != ""
}

// RouteUsesAccount reports whether Codex currently resolves through accountID.
func RouteUsesAccount(cfg configuration.Config, accountID string) bool {
	activeAccount, ok := RouteAccount(cfg)
	return ok && activeAccount == accountID
}

// AuthenticationChanged reports whether a transition requires rebinding Codex
// native authentication.
func AuthenticationChanged(before, after configuration.Config) bool {
	beforeAdapter := before.Adapters[configuration.ClientCodex]
	afterAdapter := after.Adapters[configuration.ClientCodex]
	if !afterAdapter.Enabled {
		return false
	}
	if !beforeAdapter.Enabled || !slices.Equal(beforeAdapter.Targets, afterAdapter.Targets) {
		return true
	}
	beforeAccount, beforeOK := RouteAccount(before)
	afterAccount, afterOK := RouteAccount(after)
	return afterOK && (!beforeOK || beforeAccount != afterAccount)
}
