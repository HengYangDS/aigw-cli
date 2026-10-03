package readiness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"aigw-cli/internal/configuration"
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
			client := result.Clients[configuration.ClientClaude]
			if result.OK || client.State != test.state || client.Detail != test.detail || client.NextAction != test.action || result.NextAction != test.action {
				t.Fatalf("JSON check = %+v", result)
			}
		})
	}
}
