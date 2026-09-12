// Package claude owns Claude Code command and settings projection.
package claude

import (
	"fmt"
	"strings"

	configuration "aigw-cli/internal/configuration"
	"aigw-cli/internal/process"
)

// VerificationPlan consumes synchronized settings and their credential helper
// without overriding them or loading unrelated client customizations.
func VerificationPlan(executable, settingsPath, prompt string, currentEnv []string, runtime configuration.Runtime) (process.Plan, error) {
	if executable == "" {
		return process.Plan{}, fmt.Errorf("Claude executable is not configured")
	}
	if err := ValidateSettings(settingsPath, runtime, runtime.CredentialCommand); err != nil {
		return process.Plan{}, err
	}
	return process.Plan{
		Executable: executable,
		Args:       []string{"--bare", "--settings", settingsPath, "--disable-slash-commands", "--no-session-persistence", "--tools", "", "--print", prompt},
		Env:        removeEnvironment(currentEnv, managedEnvironmentKeys...),
	}, nil
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
