package check

import (
	"slices"
	"testing"

	"aigw-cli/internal/adapter/codex"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/domain"
)

// localCheckOf returns the client's local check named name.
func localCheckOf(t *testing.T, client clientDocument, name string) localCheck {
	t.Helper()
	index := slices.IndexFunc(client.Checks, func(check localCheck) bool { return check.Name == name })
	if index < 0 {
		t.Fatalf("%s has no check %s: %+v", client.ID, name, client.Checks)
	}
	return client.Checks[index]
}

// A named client that selects no model stops at the local depth, sends no
// request and reads no Token. Its fix selects the team's recommended model,
// or else a model that the user chooses, and the Token check says why it did
// not run.
func TestCheckOfAClientWithoutAModel(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*domain.Config)
		problem string
		state   domainreadiness.State
		action  string
		detail  string
	}{
		{name: "no selection", prepare: func(cfg *domain.Config) {
			delete(cfg.Clients, codex.ID)
		}, problem: "No model is selected", state: domainreadiness.Deferred, detail: "aigw use codex <provider/model>"},
		{name: "a recommendation", prepare: func(cfg *domain.Config) {
			delete(cfg.Clients, codex.ID)
			cfg.Recommendations[codex.ID] = domain.ClientRecommendation{Primary: domain.ClientSelection{Model: "codex"}}
		}, problem: "No model is selected", state: domainreadiness.Deferred, action: "aigw use codex one/gpt-test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, out := configuredRuntime(t)
			test.prepare(&cfg)
			save(t, runtime, cfg)
			provider := &recorder{respond: answer}
			runtime.HTTP = provider
			if err := execute(runtime, "codex", "--json"); err == nil {
				t.Fatal("a client without a model passed")
			}
			client := decode(t, out).client(t, codex.ID)
			if client.Failure == nil || client.Failure.Depth != depthLocal || client.Failure.Problem != test.problem || client.State != test.state ||
				client.NextAction != test.action || client.NextDetail != test.detail {
				t.Fatalf("%s = %+v", test.name, client)
			}
			if token := localCheckOf(t, client, "codex:token"); token.State != checkSkipped || token.Detail != "No model resolves, so no provider's Token applies" {
				t.Fatalf("the Token check = %+v", token)
			}
			if len(provider.requests) != 0 || len(secrets(t, runtime).getProviders) != 0 {
				t.Fatalf("requests %q, Token reads %q", provider.requests, secrets(t, runtime).getProviders)
			}
		})
	}
}
