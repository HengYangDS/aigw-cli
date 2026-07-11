package adapters

import (
	"fmt"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func ClaudePlan(executable string, args, currentEnv []string, runtime domain.Runtime, token string) (ProcessPlan, error) {
	if executable == "" {
		return ProcessPlan{}, fmt.Errorf("Claude executable is not configured")
	}
	if token == "" {
		return ProcessPlan{}, fmt.Errorf("profile %q has no token", runtime.ProfileID)
	}
	if runtime.Endpoint == "" {
		return ProcessPlan{}, fmt.Errorf("profile %q has no Claude endpoint", runtime.ProfileID)
	}
	env := removeEnvironment(currentEnv, "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL", "AIGW_ACCOUNT", "AIGW_PROFILE")
	env = append(env,
		"ANTHROPIC_AUTH_TOKEN="+token,
		"ANTHROPIC_BASE_URL="+runtime.Endpoint,
		"AIGW_ACCOUNT="+runtime.AccountID,
		"AIGW_PROFILE="+runtime.ProfileID,
	)
	if model := runtime.Model; model != "" {
		env = append(env, "ANTHROPIC_MODEL="+model)
	}
	return ProcessPlan{Executable: executable, Args: append([]string(nil), args...), Env: env, Replace: true}, nil
}

func removeEnvironment(env []string, keys ...string) []string {
	remove := map[string]bool{}
	for _, key := range keys {
		remove[key] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !remove[key] {
			out = append(out, entry)
		}
	}
	return out
}
