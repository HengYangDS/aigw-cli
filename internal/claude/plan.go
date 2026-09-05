// Package claude owns Claude Code command and settings projection.
package claude

import (
	"fmt"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

func Plan(executable string, args, currentEnv []string, runtime configuration.Runtime, token string) (process.Plan, error) {
	if executable == "" {
		return process.Plan{}, fmt.Errorf("Claude executable is not configured")
	}
	if token == "" {
		return process.Plan{}, fmt.Errorf("profile %q has no token", runtime.ProfileID)
	}
	if runtime.Endpoint == "" {
		return process.Plan{}, fmt.Errorf("profile %q has no Claude endpoint", runtime.ProfileID)
	}
	env := removeEnvironment(currentEnv, "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS", "AIGW_ACCOUNT", "AIGW_PROFILE")
	env = append(env,
		"ANTHROPIC_AUTH_TOKEN="+token,
		"ANTHROPIC_BASE_URL="+runtime.Endpoint,
		"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1",
		"AIGW_ACCOUNT="+runtime.AccountID,
		"AIGW_PROFILE="+runtime.ProfileID,
	)
	if model := runtime.Model; model != "" {
		env = append(env, "ANTHROPIC_MODEL="+model)
	}
	return process.Plan{Executable: executable, Args: append([]string(nil), args...), Env: env, Replace: true}, nil
}

func removeEnvironment(env []string, keys ...string) []string {
	remove := map[string]bool{}
	for _, key := range keys {
		remove[key] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if remove[key] || strings.HasPrefix(key, "AIGW_TOKEN_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
