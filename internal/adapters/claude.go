package adapters

import (
	"fmt"
	"strings"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func ClaudePlan(executable string, args, currentEnv []string, profile domain.Profile, token string) (ProcessPlan, error) {
	if executable == "" {
		return ProcessPlan{}, fmt.Errorf("Claude executable is not configured")
	}
	if token == "" {
		return ProcessPlan{}, fmt.Errorf("profile %q has no token", profile.ID)
	}
	endpoint, err := profile.EndpointFor(domain.ClientClaude)
	if err != nil {
		return ProcessPlan{}, err
	}
	env := removeEnvironment(currentEnv, "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "AIGW_PROFILE")
	env = append(env,
		"ANTHROPIC_AUTH_TOKEN="+token,
		"ANTHROPIC_BASE_URL="+endpoint,
		"AIGW_PROFILE="+profile.ID,
	)
	return ProcessPlan{Executable: executable, Args: append([]string(nil), args...), Env: env}, nil
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
