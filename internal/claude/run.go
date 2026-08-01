package claude

import (
	"context"
	"fmt"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
	"aigw-cli/internal/secrets"
)

// Runner executes one prepared client process plan.
type Runner interface {
	Run(context.Context, process.Plan) error
}

// Run resolves the active Claude route, obtains its token, and replaces the
// current process through the supplied managed runner.
func Run(ctx context.Context, store configuration.Store, secretStore secrets.Store, runner Runner, args, environment []string) error {
	cfg, err := store.Load()
	if err != nil {
		return err
	}
	adapter := cfg.Adapters[configuration.ClientClaude]
	if !adapter.Enabled || adapter.Executable == "" {
		return fmt.Errorf("Claude adapter is not enabled; run `aigw adapter enable claude --executable <real-claude>`")
	}
	runtime, _, err := cfg.ResolveRuntime(configuration.ClientClaude, "")
	if err != nil {
		return err
	}
	token, err := secretStore.Get(runtime.AccountID)
	if err != nil {
		return fmt.Errorf("Token for the Claude route is unavailable: %w; run `aigw rotate %s`", err, runtime.AccountID)
	}
	plan, err := Plan(adapter.Executable, args, environment, runtime, token)
	if err != nil {
		return err
	}
	return runner.Run(ctx, plan)
}
