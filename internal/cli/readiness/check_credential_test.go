package readiness

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/diagnostics"
	domainreadiness "aigw-cli/internal/readiness"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/native"
)

func TestCheckJSONClassifiesCredentialReadFailures(t *testing.T) {
	for _, test := range []struct {
		name, detail, action string
		readError            error
		state                domainreadiness.State
	}{
		{"unreadable", "account token could not be read", domainreadiness.CredentialBackendRecovery, fmt.Errorf("private credential detail: %w", native.ErrUnavailable), domainreadiness.Unavailable},
		{"disappeared", "account token is unavailable", "aigw rotate one", secrets.ErrNotFound, domainreadiness.Deferred},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, cfg, output := configuredReadinessRuntime(t)
			runtime.Secrets = presentFailingSecretStore{err: test.readError}
			configureClaudeExecutable(t, &runtime, &cfg)
			synchronizeClaudeSettings(t, runtime, cfg)
			if err := runtime.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("credential read failure triggered a provider request")
				return nil, nil
			})
			command := NewCheckCommand(runtime)
			command.SetArgs([]string{"--for", "claude", "--json"})
			if err := executeCommand(command); err == nil {
				t.Fatal("credential read failure passed check")
			}
			if strings.Contains(output.String(), "private credential detail") {
				t.Fatalf("check exposed a private cause: %s", output.String())
			}
			var result checkJSON
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			client := result.Clients[claude.ID]
			if result.OK || client.State != test.state || client.Detail != test.detail || client.NextAction != test.action || result.NextAction != test.action {
				t.Fatalf("JSON check = %+v", result)
			}
		})
	}
}

func TestCheckReportsUnresolvedBindingsBeforeCredentialOrNetworkAccess(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	var problem, action string
	runtime.Problem = func(title, _, _, fix string, cause error) error {
		problem, action = title, fix
		return cause
	}
	store := &observingSecretStore{value: "fixture-token"}
	runtime.Secrets = store
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("an unresolved binding contacted an endpoint")
		return nil, nil
	})
	cfg.SetSelectedRoute(codex.ID, "missing", "")
	result := evaluateClient(NewCheckCommand(runtime), runtime, cfg, codex.ID)
	_, command, unchecked, err := checkedClientField(runtime, result, "")
	if err == nil || !errors.Is(err, result.resolveErr) || command != "" || unchecked {
		t.Fatalf("unresolved binding was reported as checked: %q, %t, %v", command, unchecked, err)
	}
	if problem != "Codex binding cannot be resolved" || action != "aigw use --for codex <route>" {
		t.Fatalf("binding failure omitted its actionable scope: %q, %q", problem, action)
	}
	if store.getCalls != 0 || store.existsCalls != 0 {
		t.Fatal("an unresolved binding observed credentials")
	}
}

func TestCheckReportsAnUnsentInferenceRequestWithoutClaimingEndpointFailure(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	var problem, evidence string
	runtime.Problem = func(title, detail, _, _ string, cause error) error {
		problem, evidence = title, detail
		return cause
	}
	selected, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	selected.Model = ""
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("a missing wire model sent a request")
		return nil, nil
	})
	diagnostic := diagnostics.Probe(t.Context(), runtime.HTTP, selected, "fixture-token", diagnostics.ScopeInference)
	result := evaluatedClient{client: codex.ID, runtime: selected, adapter: true, diagnostic: diagnostic}
	_, command, unchecked, err := checkedClientField(runtime, result, "")
	if err == nil || command != "" || unchecked || diagnostic.Kind != diagnostics.ModelUnresolved {
		t.Fatalf("unsent inference was reported as checked: %q, %t, %v", command, unchecked, err)
	}
	if evidence != "No request was sent" || !strings.Contains(problem, "Route has no upstream model") {
		t.Fatalf("unsent request lost its precise diagnostic: %q, %q", problem, evidence)
	}
}
