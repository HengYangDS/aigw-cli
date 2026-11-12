// Package claude owns Claude Code command and settings projection.
package claude

import (
	"fmt"
	"slices"
	"strings"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform/process"
)

// VerificationPlan consumes synchronized settings and their credential helper
// without overriding them or loading unrelated client customizations.
func VerificationPlan(executable, settingsPath, prompt string, currentEnv []string, runtime domain.Runtime) (process.Plan, error) {
	if executable == "" {
		return process.Plan{}, fmt.Errorf("Claude executable is not configured")
	}
	if err := ValidateSettings(settingsPath, runtime, runtime.CredentialCommand); err != nil {
		return process.Plan{}, err
	}
	return process.Plan{
		Executable: executable,
		Args:       []string{"--bare", "--settings", settingsPath, "--disable-slash-commands", "--no-session-persistence", "--tools", "", "--system-prompt", "You are a concise assistant.", "--print", "--output-format", "stream-json", "--verbose", prompt},
		Env:        withoutParentSession(removeEnvironment(currentEnv, managedEnvironmentKeys...)),
	}, nil
}

// withoutParentSession removes the variables through which a parent Claude
// Code session configures its own client: its marker, its CLAUDE_CODE_* and
// CLAUDE_AGENT_SDK_* settings, and the model variables that name its own
// models. The check verifies the settings file that AIGW wrote, so the client
// must not take any of these from the process that runs the check. PATH, HOME,
// proxies, locale and the client's own configuration folder stay. Names compare
// in upper case, as Windows does.
func withoutParentSession(env []string) []string {
	return slices.DeleteFunc(env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		return key == "CLAUDECODE" || key == "ANTHROPIC_SMALL_FAST_MODEL" ||
			strings.HasPrefix(key, "CLAUDE_CODE_") || strings.HasPrefix(key, "CLAUDE_AGENT_SDK_") || strings.HasPrefix(key, "ANTHROPIC_DEFAULT_")
	})
}

func removeEnvironment(env []string, keys ...string) []string {
	remove := map[string]bool{}
	for _, key := range keys {
		remove[key] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if remove[key] {
			continue
		}
		out = append(out, entry)
	}
	return out
}
