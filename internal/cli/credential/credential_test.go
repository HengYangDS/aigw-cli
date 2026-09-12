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

func TestCredentialHelperPrintsOnlyTheProjectedAccountToken(t *testing.T) {
	for _, client := range configuration.AdmittedClientIDs() {
		for _, change := range []string{"none", "token", "model", "label"} {
			t.Run(client+"/"+change, func(t *testing.T) {
				runtime, buffer := helperRuntime(t, client, true)
				args := helperArgs(t, runtime, client)
				if err := runtime.Secrets.Set("gateway", "original-token"); err != nil {
					t.Fatal(err)
				}
				want := "original-token\n"
				if change == "token" {
					if err := runtime.Secrets.Set("gateway", "rotated-token"); err != nil {
						t.Fatal(err)
					}
					want = "rotated-token\n"
				}
				cfg, err := runtime.Config.Load()
				if err != nil {
					t.Fatal(err)
				}
				profile := cfg.Profiles[client]
				switch change {
				case "model":
					profile.Model = "another-model"
				case "label":
					profile.Label = "Another label"
				}
				cfg.Profiles[client] = profile
				if err := runtime.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				command := NewCommand(runtime)
				if !command.Hidden {
					t.Fatal("credential helper must remain hidden")
				}
				if err := command.RunE(command, args); err != nil {
					t.Fatal(err)
				}
				if got := buffer.String(); got != want {
					t.Fatalf("matching projection output = %q, want %q", got, want)
				}
			})
		}
	}
}

func TestCredentialHelperRejectsStaleBindingBeforeSecretAccess(t *testing.T) {
	for _, client := range configuration.AdmittedClientIDs() {
		for _, change := range []string{"client", "account", "endpoint"} {
			t.Run(client+"/"+change, func(t *testing.T) {
				runtime, buffer := helperRuntime(t, client, true)
				cfg, err := runtime.Config.Load()
				if err != nil {
					t.Fatal(err)
				}
				projection, err := cfg.ResolveRuntime(client, "")
				if err != nil {
					t.Fatal(err)
				}
				fingerprintClient := client
				switch change {
				case "client":
					fingerprintClient = "another-client"
				case "account":
					cfg.Accounts["other"] = cfg.Accounts[projection.AccountID]
					profile := cfg.Profiles[client]
					profile.Account = "other"
					cfg.Profiles[client] = profile
				case "endpoint":
					account := cfg.Accounts[projection.AccountID]
					account.Endpoints = configuration.Endpoints{
						Anthropic:       "https://other.test",
						OpenAIResponses: "https://other.test/v1",
					}
					cfg.Accounts[projection.AccountID] = account
				}
				if err := runtime.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				store := &refusingSecretStore{}
				runtime.Secrets = store
				command := NewCommand(runtime)
				err = command.RunE(command, []string{client, projection.CredentialProjectionFingerprint(fingerprintClient)})
				if err == nil || !strings.Contains(err.Error(), "no longer matches") {
					t.Fatalf("stale projection error = %v", err)
				}
				if got := buffer.String(); got != "" || store.getCalls != 0 || store.existsCalls != 0 {
					t.Fatalf("stale projection accessed credentials: stdout=%q get=%d exists=%d", got, store.getCalls, store.existsCalls)
				}
			})
		}
	}
}

func TestClaudeCredentialHelperFailsClosedWithoutWritingStdout(t *testing.T) {
	for name := range map[string]bool{"wrong client": true, "load": true, "disabled": true, "route": true, "secret": true} {
		t.Run(name, func(t *testing.T) {
			runtime, buffer := helperRuntime(t, configuration.ClientClaude, true)
			args := helperArgs(t, runtime, configuration.ClientClaude)
			switch name {
			case "load":
				runtime.Config = configuration.NewStore(t.TempDir())
			case "disabled":
				runtime, buffer = helperRuntime(t, configuration.ClientClaude, false)
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
			args[0] = argument
			err := command.RunE(command, args)
			if err == nil || buffer.Len() != 0 {
				t.Fatalf("error=%v stdout=%q", err, buffer.String())
			}
		})
	}
}

func TestCredentialCommandRequiresProjectionFingerprintBeforeSecretAccess(t *testing.T) {
	runtime, buffer := helperRuntime(t, configuration.ClientCodex, true)
	store := &refusingSecretStore{}
	runtime.Secrets = store
	command := NewCommand(runtime)
	command.SetArgs([]string{configuration.ClientCodex})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "accepts 2 arg(s)") {
		t.Fatalf("credential command error = %v", err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 || buffer.Len() != 0 {
		t.Fatal("incomplete invocation accessed or emitted credential bytes")
	}
}

func TestCredentialHelperUsesAccountTokenLanguage(t *testing.T) {
	runtime, _ := helperRuntime(t, configuration.ClientClaude, true)
	command := NewCommand(runtime)

	if command.Short != "Read the Account Token matching a client projection" {
		t.Fatalf("short help = %q", command.Short)
	}
	err := command.RunE(command, helperArgs(t, runtime, configuration.ClientClaude))
	if err == nil || !strings.Contains(err.Error(), "claude Account Token is unavailable") || strings.Contains(err.Error(), "gateway") {
		t.Fatalf("credential error = %v", err)
	}
}

func TestClaudeCredentialHelperPropagatesOutputFailure(t *testing.T) {
	runtime, _ := helperRuntime(t, configuration.ClientClaude, true)
	if err := runtime.Secrets.Set("gateway", "secret-token"); err != nil {
		t.Fatal(err)
	}
	runtime.Out = failingWriter{}
	command := NewCommand(runtime)
	if err := command.RunE(command, helperArgs(t, runtime, configuration.ClientClaude)); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("error=%v", err)
	}
}

func helperRuntime(t *testing.T, client string, enabled bool) (invocation.Context, *bytes.Buffer) {
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
	out := &bytes.Buffer{}
	return invocation.Context{Config: store, Secrets: secrets.NewMemoryStore(), Out: out}, out
}

func helperArgs(t *testing.T, runtime invocation.Context, client string) []string {
	t.Helper()
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	projection, err := cfg.ResolveRuntime(client, "")
	if err != nil {
		t.Fatal(err)
	}
	return []string{client, projection.CredentialProjectionFingerprint(client)}
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
	runtime, buffer := helperRuntime(t, configuration.ClientCodex, true)
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

	err = command.RunE(command, helperArgs(t, runtime, configuration.ClientCodex))
	if err == nil || !strings.Contains(err.Error(), "client-owned authentication") || !strings.Contains(err.Error(), "aigw verify --for codex") {
		t.Fatalf("credential helper error = %v", err)
	}
	if store.getCalls != 0 || store.existsCalls != 0 {
		t.Fatalf("credential helper accessed client-native credentials: get=%d exists=%d", store.getCalls, store.existsCalls)
	}
	if buffer.Len() != 0 {
		t.Fatalf("credential helper wrote stdout: %q", buffer.String())
	}
}
