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

// userVariables are the CLAUDE_CODE_* settings that the user sets for their
// own client and that a check passes on: the client certificate of mutual
// TLS, the proxy's host resolution, the shell and the temporary folder, the
// credential helper's lifetime, and the choice of another provider, with
// which the check fails as real use does. A Claude Desktop parent sets the
// provider choices empty.
var userVariables = []string{
	"CLAUDE_CODE_CLIENT_CERT", "CLAUDE_CODE_CLIENT_KEY", "CLAUDE_CODE_CLIENT_KEY_PASSPHRASE",
	"CLAUDE_CODE_PROXY_RESOLVES_HOSTS", "CLAUDE_CODE_GIT_BASH_PATH", "CLAUDE_CODE_SHELL", "CLAUDE_CODE_SHELL_PREFIX",
	"CLAUDE_CODE_TMPDIR", "CLAUDE_CODE_API_KEY_HELPER_TTL_MS",
	"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY",
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH", "CLAUDE_CODE_SKIP_VERTEX_AUTH", "CLAUDE_CODE_SKIP_FOUNDRY_AUTH",
}

// withoutParentSession removes the variables through which a parent Claude
// Code session configures its own client: its marker, its effort and process
// ID, every CLAUDE_CODE_* name but the user's own, so that a host or session
// variable that a later release adds stays out too, its CLAUDE_AGENT_SDK_*
// settings, and the model variables that name its own models. The check
// verifies the settings file that AIGW wrote, so the client must not take any
// of these from the process that runs the check. PATH, HOME, proxies, locale,
// the client's own configuration folder and userVariables stay. Names compare
// in upper case, as Windows does.
func withoutParentSession(env []string) []string {
	return slices.DeleteFunc(env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "CLAUDE_CODE_") {
			return !slices.Contains(userVariables, key)
		}
		return key == "CLAUDECODE" || key == "CLAUDE_EFFORT" || key == "CLAUDE_PID" || key == "ANTHROPIC_SMALL_FAST_MODEL" ||
			strings.HasPrefix(key, "CLAUDE_AGENT_SDK_") || strings.HasPrefix(key, "ANTHROPIC_DEFAULT_")
	})
}

// removeEnvironment removes keys from env in any case, as withoutParentSession
// compares names: on Windows a variable's name ignores case, so the client
// would read anthropic_auth_token as ANTHROPIC_AUTH_TOKEN.
func removeEnvironment(env []string, keys ...string) []string {
	remove := map[string]bool{}
	for _, key := range keys {
		remove[strings.ToUpper(key)] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if remove[strings.ToUpper(key)] {
			continue
		}
		out = append(out, entry)
	}
	return out
}
