package credential

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/secrets"
)

func TestCredentialHelperPrintsOnlyTheActiveClientToken(t *testing.T) {
	for _, client := range configuration.AdmittedClientIDs() {
		t.Run(client, func(t *testing.T) {
			runtime := helperRuntime(t, client, true)
			if err := runtime.Secrets.Set("gateway", "secret-token"); err != nil {
				t.Fatal(err)
			}
			command := NewCommand(runtime)
			if !command.Hidden {
				t.Fatal("credential helper must remain hidden")
			}
			if err := command.RunE(command, []string{client}); err != nil {
				t.Fatal(err)
			}
			if got := runtime.Out.(*bytes.Buffer).String(); got != "secret-token\n" {
				t.Fatalf("stdout=%q", got)
			}
		})
	}
}

func TestClaudeCredentialHelperFailsClosedWithoutWritingStdout(t *testing.T) {
	for name := range map[string]bool{"wrong client": true, "load": true, "disabled": true, "route": true, "secret": true} {
		t.Run(name, func(t *testing.T) {
			runtime := helperRuntime(t, configuration.ClientClaude, true)
			switch name {
			case "load":
				runtime.Config = configuration.NewStore(t.TempDir())
			case "disabled":
				runtime = helperRuntime(t, configuration.ClientClaude, false)
			case "route":
				cfg := configuration.NewConfig()
				cfg.Accounts["gateway"] = configuration.Account{Label: "Gateway", Endpoints: configuration.Endpoints{OpenAIResponses: "https://gateway.test/v1"}}
				cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "gateway", Client: configuration.ClientCodex, Model: "gpt"}
				cfg.Routes[configuration.ClientCodex] = "codex"
				cfg.Adapters[configuration.ClientClaude] = configuration.AdapterConfig{Enabled: true, Executable: "claude"}
				if err := runtime.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
			}
			argument := configuration.ClientClaude
			if name == "wrong client" {
				argument = "unsupported"
			}
			command := NewCommand(runtime)
			err := command.RunE(command, []string{argument})
			if err == nil || runtime.Out.(*bytes.Buffer).Len() != 0 {
				t.Fatalf("error=%v stdout=%q", err, runtime.Out.(*bytes.Buffer).String())
			}
		})
	}
}

func TestClaudeCredentialHelperPropagatesOutputFailure(t *testing.T) {
	runtime := helperRuntime(t, configuration.ClientClaude, true)
	if err := runtime.Secrets.Set("gateway", "secret-token"); err != nil {
		t.Fatal(err)
	}
	runtime.Out = failingWriter{}
	command := NewCommand(runtime)
	if err := command.RunE(command, []string{configuration.ClientClaude}); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("error=%v", err)
	}
}

func helperRuntime(t *testing.T, client string, enabled bool) invocation.Context {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"))
	cfg := configuration.NewConfig()
	account := configuration.Account{Label: "Gateway"}
	if client == configuration.ClientClaude {
		account.Endpoints.Anthropic = "https://gateway.test"
	} else {
		account.Endpoints.OpenAIResponses = "https://gateway.test/v1"
	}
	cfg.Accounts["gateway"] = account
	cfg.Profiles[client] = configuration.Profile{Label: client, Account: "gateway", Client: client, Model: client + "-team"}
	cfg.Routes[client] = client
	cfg.Adapters[client] = configuration.AdapterConfig{Enabled: enabled, Executable: client}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return invocation.Context{Config: store, Secrets: secrets.NewMemoryStore(), Out: &bytes.Buffer{}}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

type refusingSecretStore struct {
	getCalls    int
	existsCalls int
}

func (store *refusingSecretStore) Get(string) (string, error) {
	store.getCalls++
	return "", errors.New("credential access is forbidden")
}

func (*refusingSecretStore) Set(string, string) error { return nil }
func (*refusingSecretStore) Delete(string) error      { return nil }

func (store *refusingSecretStore) Exists(string) (bool, error) {
	store.existsCalls++
	return false, errors.New("credential observation is forbidden")
}

func TestCredentialHelperRejectsClientNativeBeforeSecretAccess(t *testing.T) {
	runtime := helperRuntime(t, configuration.ClientCodex, true)
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles[configuration.ClientCodex]
	profile.ModelProvider = "amazon-bedrock"
	profile.Authentication = configuration.AuthenticationClientNative
	cfg.Profiles[configuration.ClientCodex] = profile
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &refusingSecretStore{}
	runtime.Secrets = store
	command := NewCommand(runtime)

	err = command.RunE(command, []string{configuration.ClientCodex})
	if err == nil || !strings.Contains(err.Error(), "client-owned authentication") || !strings.Contains(err.Error(), "aigw verify --for codex") {
		t.Fatalf("credential helper error = %v", err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 {
		t.Fatalf("credential helper accessed client-native credentials: get=%d exists=%d", store.getCalls, store.existsCalls)
	}
	if runtime.Out.(*bytes.Buffer).Len() != 0 {
		t.Fatalf("credential helper wrote stdout: %q", runtime.Out.(*bytes.Buffer).String())
	}
}
