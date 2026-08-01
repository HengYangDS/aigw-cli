package claude_test

import (
	"testing"

	"aigw-cli/internal/claude"
	configuration "aigw-cli/internal/configuration"
)

func TestClaudePlanInjectsOnlyProcessLocalAnthropicVariables(t *testing.T) {
	runtime := configuration.Runtime{ProfileID: "dmx", AccountID: "dmx", Endpoint: "https://example.test"}
	plan, err := claude.Plan("/usr/local/bin/claude-real", []string{"--version"}, []string{
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
	runtime := configuration.Runtime{ProfileID: "claude-opus", AccountID: "dmx", Endpoint: "https://example.test", Model: "claude-opus"}
	plan, err := claude.Plan("/bin/claude", nil, []string{"ANTHROPIC_MODEL=old"}, runtime, "token")
	if err != nil {
		t.Fatal(err)
	}
	env := envMap(plan.Env)
	if env["ANTHROPIC_MODEL"] != "claude-opus" || env["AIGW_ACCOUNT"] != "dmx" {
		t.Fatalf("env = %#v", env)
	}
}

func TestClaudePlanErrors(t *testing.T) {
	cases := []struct {
		name       string
		executable string
		runtime    configuration.Runtime
		token      string
		wantErr    string
	}{
		{"missing executable", "", configuration.Runtime{}, "tok", "Claude executable is not configured"},
		{"missing token", "bin", configuration.Runtime{ProfileID: "p"}, "", "profile \"p\" has no token"},
		{"missing endpoint", "bin", configuration.Runtime{ProfileID: "p"}, "tok", "profile \"p\" has no Claude endpoint"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := claude.Plan(c.executable, nil, nil, c.runtime, c.token)
			if err == nil || err.Error() != c.wantErr {
				t.Fatalf("got err = %v, want %q", err, c.wantErr)
			}
		})
	}
}
