package check

import (
	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	domainreadiness "aigw-cli/internal/app/readiness"
	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCheckCommandDescribesItsProductBoundary(t *testing.T) {
	command := NewCommand(invocation.Context{Clients: clients.Registry()})
	if command.Short != "Check enabled clients; selected-model probes may use quota" {
		t.Fatalf("check summary = %q", command.Short)
	}
}

type failingOutputWriter struct{ err error }

func (writer failingOutputWriter) Write([]byte) (int, error) { return 0, writer.err }

type presentFailingSecretStore struct{ err error }

func (store presentFailingSecretStore) Get(string) (string, error) { return "", store.err }

func (presentFailingSecretStore) Set(string, string) error { return nil }

func (presentFailingSecretStore) Delete(string) error { return nil }

func (presentFailingSecretStore) Exists(string) (bool, error) { return true, nil }

type failingAccountObservationStore struct {
	err   error
	reads int
}

func (store *failingAccountObservationStore) Get(string) (secret.DiagnosticCredential, error) {
	store.reads++
	return secret.DiagnosticCredential{}, store.err
}

func (*failingAccountObservationStore) Set(string, secret.DiagnosticCredential) error { return nil }

func (*failingAccountObservationStore) Delete(string) error { return nil }

func (store *failingAccountObservationStore) Exists(string) (bool, error) {
	store.reads++
	return false, store.err
}

func TestCheckEvaluationClientLookupDistinguishesMissingClient(t *testing.T) {
	evaluation := checkEvaluation{clients: []evaluatedClient{{client: claude.ID}}}
	if client, ok := evaluation.client(claude.ID); !ok || client.client != claude.ID {
		t.Fatalf("Claude client = %#v, %v", client, ok)
	}
	if client, ok := evaluation.client(codex.ID); ok || client != (evaluatedClient{}) {
		t.Fatalf("missing Codex client = %#v, %v", client, ok)
	}
}

func TestCheckReadsEachEnabledRouteCredentialOnce(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	store := &observingSecretStore{value: "token"}
	runtime.Secrets = store
	configureClaudeExecutable(t, &runtime, &cfg)
	synchronizeClaudeSettings(t, runtime, cfg)
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(codex.ID, true, "codex", []string{target})
	clientRuntime, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	clientRuntime.CredentialCommand = runtime.Executable
	if err := codex.SyncConfig(target, clientRuntime); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	runtime.HTTP = roundTripFunc(successfulReadinessResponse)
	command := NewCommand(runtime)
	command.SetContext(t.Context())
	if evaluation := evaluateCheck(command, runtime, cfg); !evaluation.ok() {
		t.Fatalf("ready clients failed evaluation: %+v", evaluation.clients)
	}
	if store.existsCalls != 0 || store.getCalls != len([]string{claude.ID, codex.ID}) {
		t.Fatalf("exists calls=%d get calls=%d", store.existsCalls, store.getCalls)
	}
}

func TestCheckDefersUnprojectedClientWithoutReadingTokenValues(t *testing.T) {
	for _, client := range []string{claude.ID, codex.ID} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", client, jsonMode), func(t *testing.T) {
				runtime, cfg, output := configuredReadinessRuntime(t)
				runtime.Version = "1.0.0"
				problemDetail, problemAction := "", ""
				runtime.Problem = func(_, evidence, _, action string, cause error) error {
					problemDetail, problemAction = evidence, action
					return cause
				}
				cfg.SetClientActivation(client, true, "", nil)
				if err := runtime.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				store := &observingSecretStore{value: "available-token", getErr: errors.New("secret read before projection admission")}
				runtime.Secrets = store
				runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
					t.Fatal("invalid projection must not authenticate an endpoint")
					return nil, nil
				})
				command := NewCommand(runtime)
				if jsonMode {
					command.SetArgs([]string{"--json"})
				} else {
					command.SetArgs([]string{})
				}
				if err := executeCommand(command); err == nil {
					t.Fatal("invalid projection was accepted")
				}
				if store.getCalls != 0 || store.existsCalls != 1 {
					t.Fatalf("deferred projection observed Token value or repeated metadata lookup: get=%d exists=%d", store.getCalls, store.existsCalls)
				}
				if jsonMode {
					var result checkDocument
					if err := json.Unmarshal(output.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					problemDetail, problemAction = result.Clients[client].Detail, result.Clients[client].NextAction
					if state := result.Clients[client]; state.State != domainreadiness.Deferred || state.NextAction != problemAction || state.Detail != problemDetail {
						t.Fatalf("client status = %+v", state)
					}
				}
				if !strings.Contains(strings.ToLower(problemDetail), "projection") || problemAction != "Install "+clients.Admission().Label(client)+" if needed, then run `aigw sync`" {
					t.Fatalf("projection recovery was lost: detail=%q action=%q", problemDetail, problemAction)
				}
			})
		}
	}
}

func TestCheckExplainsBothMissingTokenAndDeferredProjection(t *testing.T) {
	runtime, cfg, output := configuredReadinessRuntime(t)
	cfg.SetClientActivation(claude.ID, true, "", nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err == nil {
		t.Fatal("missing Token and projection were accepted")
	}
	var result checkDocument
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.NextAction != "run `aigw rotate one`" || !strings.Contains(result.Error, "Token") || !strings.Contains(result.Error, "projection") {
		t.Fatalf("missing prerequisites were not explained together: %+v", result)
	}
}

func TestCheckReportsCredentialMetadataFailureBeforeClientProjection(t *testing.T) {
	runtime, cfg, output := configuredReadinessRuntime(t)
	cfg.SetClientActivation(claude.ID, true, "", nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{existsErr: errors.New("metadata denied")}
	runtime.Secrets = store
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unavailable credential metadata must not trigger an endpoint probe")
		return nil, nil
	})
	command := NewCommand(runtime)
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err == nil {
		t.Fatal("credential metadata failure was accepted")
	}
	var result checkDocument
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != domainreadiness.Unavailable || result.NextAction != "aigw doctor" || result.Clients[claude.ID].State != domainreadiness.Unavailable {
		t.Fatalf("metadata failure = %+v", result)
	}
	if store.getCalls != 0 || store.existsCalls != 1 {
		t.Fatalf("metadata failure read Token or repeated observation: get=%d exists=%d", store.getCalls, store.existsCalls)
	}
}

func TestJSONCheckSelectsTheFailedClientRecoveryAction(t *testing.T) {
	runtime, cfg, output := configuredReadinessRuntime(t)
	cfg.SetClientActivation(claude.ID, true, filepath.Join(t.TempDir(), "missing-claude"), nil)
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	command := NewCommand(runtime)
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); err == nil {
		t.Fatal("missing native client was accepted")
	}
	var result checkDocument
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	client := result.Clients[claude.ID]
	if client.NextAction == "" || result.NextAction != client.NextAction {
		t.Fatalf("top-level recovery differs from the failed client: %+v", result)
	}
}

func TestCheckHonorsClientNativeAuthenticationOwnership(t *testing.T) {
	runtime, cfg, buffer := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	binding := cfg.Clients[codex.ID]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = domain.AuthenticationClientNative
	delete(cfg.Clients, claude.ID)
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	binding.Enabled = true
	binding.Executable = "codex"
	binding.Targets = []string{target}
	cfg.Clients[codex.ID] = binding
	clientRuntime, err := cfg.ResolveRuntime(codex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := codex.SyncConfig(target, clientRuntime); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &observingSecretStore{getErr: errors.New("client-native credential access")}
	runtime.Secrets = store
	requests := 0
	runtime.HTTP = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("client-native endpoint probe")
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())

	if err := RunCheck(command, runtime); err != nil {
		t.Fatalf("client-native check error = %v", err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native check used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
	human := buffer.String()
	for _, want := range []string{"Local projection checked", "Client-owned authentication", "aigw verify --for codex", "All enabled client checks passed", "Model inference was not verified", "Real-client execution was not verified"} {
		if !strings.Contains(strings.ToLower(human), strings.ToLower(want)) {
			t.Fatalf("client-native check output = %q, want %q", human, want)
		}
	}
	for _, forbidden := range []string{"Every enabled client route is healthy", "remote authentication succeeded", "model request succeeded"} {
		if strings.Contains(human, forbidden) {
			t.Fatalf("client-native check output = %q, contains %q", human, forbidden)
		}
	}

	buffer.Reset()
	if err := runJSONCheck(command, runtime); err != nil {
		t.Fatalf("client-native check --json error = %v", err)
	}
	var machine struct {
		OK      bool                                  `json:"ok"`
		Clients map[string]map[string]json.RawMessage `json:"clients"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &machine); err != nil {
		t.Fatal(err)
	}
	want := map[string]json.RawMessage{
		"state":               json.RawMessage(`"configured"`),
		"route":               json.RawMessage(`"codex"`),
		"account":             json.RawMessage(`"one"`),
		"detail":              json.RawMessage(`"Projection ready; client-owned authentication is not proven"`),
		"authentication":      json.RawMessage(`"client-native"`),
		"endpoint_configured": json.RawMessage(`true`),
		"projection_ready":    json.RawMessage(`true`),
		"check_passed":        json.RawMessage(`true`),
		"next_action":         json.RawMessage(`"aigw verify --for codex"`),
	}
	if !machine.OK || !reflect.DeepEqual(machine.Clients[codex.ID], want) {
		t.Fatalf("client-native JSON check = %#v", machine)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || requests != 0 {
		t.Fatalf("client-native JSON check used AIGW authentication capabilities: get=%d exists=%d HTTP=%d", store.getCalls, store.existsCalls, requests)
	}
}

func TestCheckJSONReportsOutputFailure(t *testing.T) {
	runtime, cfg, _ := configuredReadinessRuntime(t)
	runtime.Version = "1.0.0"
	cfg.Clients = map[string]domain.ClientBinding{}
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	want := errors.New("output failed")
	runtime.Out = failingOutputWriter{err: want}
	command := NewCommand(runtime)
	command.SetArgs([]string{"--json"})
	if err := executeCommand(command); !errors.Is(err, want) {
		t.Fatalf("check --json output error = %v", err)
	}
}

func TestRunCheckCoversClientResolutionAndProjectionFailures(t *testing.T) {
	t.Run("no enabled clients", func(t *testing.T) {
		runtime, _, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		if err := RunCheck(&cobra.Command{}, runtime); err == nil || !strings.Contains(err.Error(), "no enabled Client Bindings") {
			t.Fatalf("RunCheck() error = %v", err)
		}
	})

	t.Run("token lookup failure", func(t *testing.T) {
		runtime, cfg, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		want := errors.New("credential backend unavailable")
		runtime.Secrets = presentFailingSecretStore{err: want}
		configureClaudeExecutable(t, &runtime, &cfg)
		synchronizeClaudeSettings(t, runtime, cfg)
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := RunCheck(&cobra.Command{}, runtime); !errors.Is(err, want) {
			t.Fatalf("RunCheck() error = %v, want %v", err, want)
		}
	})

	t.Run("Codex projection drift", func(t *testing.T) {
		runtime, cfg, _ := configuredReadinessRuntime(t)
		runtime.Version = "1.0.0"
		if err := runtime.Secrets.Set("one", "token"); err != nil {
			t.Fatal(err)
		}
		binding := cfg.Clients[codex.ID]
		binding.Enabled = true
		binding.Executable = "/opt/codex"
		binding.Targets = []string{filepath.Join(t.TempDir(), "missing.toml")}
		cfg.Clients[codex.ID] = binding
		if err := runtime.Config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		if err := RunCheck(&cobra.Command{}, runtime); err == nil || !strings.Contains(err.Error(), "projection not ready") {
			t.Fatalf("RunCheck() error = %v", err)
		}
	})
}
