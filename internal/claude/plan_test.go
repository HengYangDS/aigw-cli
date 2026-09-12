package claude_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
)

func TestVerificationPlanConsumesTheSynchronizedSettings(t *testing.T) {
	settings, runtime := verificationSettings(t)
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	retained := []string{"PATH=/usr/bin", "AIGW_SECRET_BACKEND=env", "AIGW_TOKEN_GATEWAY=fixture-token", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1"}
	environment := append(slices.Clone(retained), "ANTHROPIC_API_KEY=stale", "ANTHROPIC_AUTH_TOKEN=stale", "ANTHROPIC_BASE_URL=stale", "ANTHROPIC_MODEL=stale")
	plan, err := claude.VerificationPlan("claude", settings, "AIGW_OK", environment, runtime)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--bare", "--settings", settings, "--disable-slash-commands", "--no-session-persistence", "--tools", "", "--print", "AIGW_OK"}
	if plan.Executable != "claude" || !slices.Equal(plan.Args, wantArgs) || !slices.Equal(plan.Env, retained) {
		t.Fatalf("plan = %#v", plan)
	}
	after, err := os.ReadFile(settings)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("verification changed settings: %v", err)
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
				runtime.Model = "changed-model"
			}
			_, err := claude.VerificationPlan("claude", settings, "AIGW_OK", nil, runtime)
			if err == nil || !strings.Contains(err.Error(), "not synchronized") || !strings.Contains(err.Error(), "aigw sync") {
				t.Fatalf("settings error = %v", err)
			}
		})
	}
}

func TestVerificationPlanRequiresExecutable(t *testing.T) {
	_, err := claude.VerificationPlan("", "", "", nil, configuration.Runtime{})
	if err == nil || !strings.Contains(err.Error(), "executable is not configured") {
		t.Fatalf("executable error = %v", err)
	}
}

func verificationSettings(t *testing.T) (string, configuration.Runtime) {
	t.Helper()
	root := t.TempDir()
	runtime := configuration.Runtime{ProfileID: "claude", AccountID: "gateway", Endpoint: "https://example.test", Model: "claude-sonnet-4-6", CredentialCommand: filepath.Join(root, "aigw")}
	settings := filepath.Join(root, "settings.json")
	if _, err := claude.ReconcileSettings(settings, false, runtime, runtime.CredentialCommand, runtime.Model); err != nil {
		t.Fatal(err)
	}
	return settings, runtime
}
