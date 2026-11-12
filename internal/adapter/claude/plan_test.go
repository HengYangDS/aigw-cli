package claude_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/domain"
)

func TestVerificationPlanConsumesTheSynchronizedSettings(t *testing.T) {
	settings, runtime := verificationSettings(t)
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	retained := []string{"PATH=/usr/bin", "AIGW_SECRET_BACKEND=env", "AIGW_TOKEN_GATEWAY=fixture-token", "HOME=/home/user", "HTTPS_PROXY=http://proxy.test", "LANG=C"}
	environment := append(slices.Clone(retained), "ANTHROPIC_API_KEY=stale", "ANTHROPIC_AUTH_TOKEN=stale", "ANTHROPIC_BASE_URL=stale", "ANTHROPIC_MODEL=stale")
	plan, err := claude.VerificationPlan("claude", settings, "AIGW_OK", environment, runtime)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--bare", "--settings", settings, "--disable-slash-commands", "--no-session-persistence", "--tools", "", "--system-prompt", "You are a concise assistant.", "--print", "--output-format", "stream-json", "--verbose", "AIGW_OK"}
	if plan.Executable != "claude" || !slices.Equal(plan.Args, wantArgs) || !slices.Equal(plan.Env, retained) {
		t.Fatalf("plan = %#v", plan)
	}
	after, err := os.ReadFile(settings)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("verification changed settings: %v", err)
	}
}

// A check run from inside a Claude Code session starts the client without
// the variables through which that session configures its own client: its
// marker, its host and session variables, any other CLAUDE_CODE_* name, such
// as one that a later release adds, and its model mapping. The user's own
// connection, runtime and provider-choice settings stay, set or empty, as a
// Claude Desktop parent leaves the provider choices, and so does the rest of
// the user's environment.
func TestVerificationPlanDropsAParentSessionsVariables(t *testing.T) {
	settings, runtime := verificationSettings(t)
	retained := []string{"PATH=/usr/bin", "HOME=/home/user", "HTTPS_PROXY=http://proxy.test", "LANG=C", "ANTHROPIC_CUSTOM_HEADERS=x: y", "CLAUDE_CONFIG_DIR=/home/user/.claude",
		"CLAUDE_CODE_CLIENT_CERT=/home/user/client.pem", "CLAUDE_CODE_CLIENT_KEY=/home/user/client-key.pem", "CLAUDE_CODE_CLIENT_KEY_PASSPHRASE=x",
		"CLAUDE_CODE_PROXY_RESOLVES_HOSTS=1", "CLAUDE_CODE_GIT_BASH_PATH=/git/bin/bash", "CLAUDE_CODE_SHELL=/bin/zsh", "CLAUDE_CODE_SHELL_PREFIX=/usr/bin/time",
		"CLAUDE_CODE_TMPDIR=/home/user/tmp", "CLAUDE_CODE_API_KEY_HELPER_TTL_MS=60000",
		"CLAUDE_CODE_USE_BEDROCK=", "CLAUDE_CODE_USE_VERTEX=", "CLAUDE_CODE_USE_FOUNDRY=1",
		"CLAUDE_CODE_SKIP_BEDROCK_AUTH=", "CLAUDE_CODE_SKIP_VERTEX_AUTH=", "CLAUDE_CODE_SKIP_FOUNDRY_AUTH=1"}
	parent := []string{"CLAUDECODE=1", "CLAUDE_CODE_ENTRYPOINT=cli", "CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1", "CLAUDE_CODE_HOST_AUTH_ENV_VAR=HOST_TOKEN",
		"CLAUDE_CODE_CHILD_SESSION=1", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1", "CLAUDE_CODE_FUTURE_HOST_FLAG=1", "CLAUDE_CODE_USE_GATEWAY=1",
		"claude_code_session_id=lower", "CLAUDE_AGENT_SDK_VERSION=1", "CLAUDE_EFFORT=high", "CLAUDE_PID=4242",
		"ANTHROPIC_DEFAULT_OPUS_MODEL=opus", "ANTHROPIC_DEFAULT_SONNET_MODEL=sonnet", "ANTHROPIC_SMALL_FAST_MODEL=haiku"}
	plan, err := claude.VerificationPlan("claude", settings, "AIGW_OK", slices.Concat(parent, retained), runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Env, retained) {
		t.Fatalf("environment = %q, want %q", plan.Env, retained)
	}
}

// A managed key that the environment spells in another case is removed too,
// as Windows, where a variable's name ignores case, would hand it to the
// client; the parent session's variables compare the same way.
func TestVerificationPlanRemovesAManagedKeyInAnyCase(t *testing.T) {
	settings, runtime := verificationSettings(t)
	retained := []string{"PATH=/usr/bin", "HOME=/home/user"}
	managed := []string{"anthropic_auth_token=stale", "Anthropic_Base_Url=stale", "anthropic_api_key=stale", "anthropic_model=stale"}
	plan, err := claude.VerificationPlan("claude", settings, "AIGW_OK", slices.Concat(managed, retained), runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Env, retained) {
		t.Fatalf("environment = %q, want %q", plan.Env, retained)
	}
}

func TestVerificationPlanRequiresConvergedSettings(t *testing.T) {
	for _, state := range []string{"missing", "malformed", "stale"} {
		t.Run(state, func(t *testing.T) {
			settings, runtime := verificationSettings(t)
			switch state {
			case "missing":
				settings = filepath.Join(t.TempDir(), "absent.json")
			case "malformed":
				if err := os.WriteFile(settings, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "stale":
				runtime.UpstreamModel = "changed-model"
			}
			_, err := claude.VerificationPlan("claude", settings, "AIGW_OK", nil, runtime)
			if err == nil || !strings.Contains(err.Error(), "not synchronized") {
				t.Fatalf("settings error = %v", err)
			}
			// Synchronization repairs missing or stale settings, and refuses a
			// file that does not parse, whose error names the correction.
			if _, refused := errors.AsType[*adapter.ConflictError](err); refused != (state == "malformed") || strings.Contains(err.Error(), "aigw sync") == refused {
				t.Fatalf("settings error = %v, refused by synchronization = %t", err, refused)
			}
		})
	}
}

func TestVerificationPlanRequiresExecutable(t *testing.T) {
	_, err := claude.VerificationPlan("", "", "", nil, domain.Runtime{})
	if err == nil || !strings.Contains(err.Error(), "executable is not configured") {
		t.Fatalf("executable error = %v", err)
	}
}

func verificationSettings(t *testing.T) (string, domain.Runtime) {
	t.Helper()
	root := t.TempDir()
	runtime := domain.Runtime{Model: domain.NewModelRef("gateway", "claude-sonnet-4-6"), ProviderID: "gateway", Endpoint: "https://example.test", UpstreamModel: "claude-sonnet-4-6", CredentialCommand: filepath.Join(root, "aigw")}
	settings := filepath.Join(root, "settings.json")
	if _, err := claude.ReconcileSettings(settings, false, runtime, runtime.CredentialCommand, runtime.UpstreamModel); err != nil {
		t.Fatal(err)
	}
	return settings, runtime
}
