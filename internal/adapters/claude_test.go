package adapters_test

import (
	"testing"

	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/adapters"
	"gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/domain"
)

func TestClaudePlanInjectsOnlyProcessLocalAnthropicVariables(t *testing.T) {
	runtime := domain.Runtime{ProfileID: "dmx", AccountID: "dmx", Endpoint: "https://example.test"}
	plan, err := adapters.ClaudePlan("/usr/local/bin/claude-real", []string{"--version"}, []string{
		"PATH=/usr/bin", "ANTHROPIC_API_KEY=stale", "ANTHROPIC_AUTH_TOKEN=stale", "ANTHROPIC_BASE_URL=stale",
		"AIGW_TOKEN_AIHUBMIX=unrelated", "AIGW_TOKEN_DMXAPI=unrelated",
	}, runtime, "fresh-secret")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Executable != "/usr/local/bin/claude-real" || len(plan.Args) != 1 {
		t.Fatalf("plan = %#v", plan)
	}
	if !plan.Replace {
		t.Fatal("Claude launch must replace the AIGW process")
	}
	env := envMap(plan.Env)
	if env["ANTHROPIC_AUTH_TOKEN"] != "fresh-secret" || env["ANTHROPIC_BASE_URL"] != "https://example.test" {
		t.Fatalf("env = %#v", env)
	}
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Fatal("stale ANTHROPIC_API_KEY survived")
	}
	if _, ok := env["AIGW_TOKEN_AIHUBMIX"]; ok {
		t.Fatal("unrelated AIHubMix Token survived")
	}
	if _, ok := env["AIGW_TOKEN_DMXAPI"]; ok {
		t.Fatal("unrelated DMXAPI Token survived")
	}
	if env["AIGW_PROFILE"] != "dmx" {
		t.Fatalf("AIGW_PROFILE = %q", env["AIGW_PROFILE"])
	}
}

func envMap(values []string) map[string]string {
	out := map[string]string{}
	for _, value := range values {
		for i := 0; i < len(value); i++ {
			if value[i] == '=' {
				out[value[:i]] = value[i+1:]
				break
			}
		}
	}
	return out
}

func TestClaudePlanProjectsModelWhenConfigured(t *testing.T) {
	runtime := domain.Runtime{ProfileID: "claude-opus", AccountID: "dmx", Endpoint: "https://example.test", Model: "claude-opus"}
	plan, err := adapters.ClaudePlan("/bin/claude", nil, []string{"ANTHROPIC_MODEL=old"}, runtime, "token")
	if err != nil {
		t.Fatal(err)
	}
	env := envMap(plan.Env)
	if env["ANTHROPIC_MODEL"] != "claude-opus" || env["AIGW_ACCOUNT"] != "dmx" {
		t.Fatalf("env = %#v", env)
	}
}
