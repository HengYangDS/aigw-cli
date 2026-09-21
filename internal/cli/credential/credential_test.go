package credential

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
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

func TestCredentialHelperResolvesAnUnselectedProfileProjection(t *testing.T) {
	for _, client := range configuration.AdmittedClientIDs() {
		t.Run(client, func(t *testing.T) {
			runtime, buffer := helperRuntime(t, client, true)
			cfg, err := runtime.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			account := configuration.Account{Label: "Alternate"}
			if client == configuration.ClientClaude {
				account.Endpoints.Anthropic = "https://alternate.test"
			} else {
				account.Endpoints.OpenAIResponses = "https://alternate.test/v1"
			}
			cfg.Accounts["alternate"] = account
			cfg.Profiles["alternate"] = configuration.Profile{
				Label: "Alternate", Account: "alternate", Model: client + "-alternate",
			}
			if err := runtime.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			if err := runtime.Secrets.Set("alternate", "alternate-token"); err != nil {
				t.Fatal(err)
			}
			projection, err := cfg.ResolveRuntime(client, "alternate")
			if err != nil {
				t.Fatal(err)
			}
			command := NewCommand(runtime)
			err = command.RunE(command, []string{client, projection.CredentialProjectionFingerprint(client)})
			if err != nil {
				t.Fatal(err)
			}
			if got := buffer.String(); got != "alternate-token\n" {
				t.Fatalf("unselected Profile credential = %q", got)
			}
		})
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
				cfg, err := runtime.Config.Load()
				if err != nil {
					t.Fatal(err)
				}
				profile := cfg.Profiles[configuration.ClientClaude]
				profile.Model = "changed-after-projection"
				cfg.Profiles[configuration.ClientClaude] = profile
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
	writeErr := errors.New("write failed")
	runtime.Out = failingWriter{err: writeErr}
	command := NewCommand(runtime)
	if err := command.RunE(command, helperArgs(t, runtime, configuration.ClientClaude)); !errors.Is(err, writeErr) {
		t.Fatalf("error=%v", err)
	}
}

func TestCredentialHelperRedactsBackendDiagnostics(t *testing.T) {
	runtime, stdout := helperRuntime(t, configuration.ClientCodex, true)
	runtime.Secrets = &refusingSecretStore{}
	command := NewCommand(runtime)
	err := command.RunE(command, helperArgs(t, runtime, configuration.ClientCodex))
	var stderr bytes.Buffer
	renderer := presentation.New(&stderr, false)
	presentation.RenderError(renderer, err, false)
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Account Token is unavailable") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "credential access is forbidden") {
		t.Fatalf("raw backend error escaped the credential boundary: %q", stderr.String())
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
	spec, _ := configuration.ClientSpecFor(client)
	cfg.Accounts["gateway"] = account
	cfg.Profiles[client] = configuration.Profile{Label: client, Account: "gateway", Model: client + "-team"}
	cfg.Clients[client] = configuration.ClientBinding{Profile: client, Enabled: enabled, Protocol: spec.EndpointProtocols[0], Executable: client}
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

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

type refusingSecretStore struct {
	getCalls    int
	existsCalls int
	readError   error
}

func (store *refusingSecretStore) Get(string) (string, error) {
	store.getCalls++
	if store.readError != nil {
		return "", store.readError
	}
	return "", errors.New("credential access is forbidden")
}

func TestCredentialReadDeadlineHasPreciseRecovery(t *testing.T) {
	runtime, stdout := helperRuntime(t, configuration.ClientCodex, true)
	runtime.Secrets = &refusingSecretStore{readError: context.DeadlineExceeded}
	command := NewCommand(runtime)
	err := command.RunE(command, helperArgs(t, runtime, configuration.ClientCodex))
	var diagnostic bytes.Buffer
	presentation.RenderCredentialError(presentation.New(&diagnostic, false), err)
	if err == nil || stdout.Len() != 0 || !strings.Contains(diagnostic.String(), "deadline") || strings.Contains(diagnostic.String(), "aigw sync") {
		t.Fatalf("read diagnostic is not specific: %v, %q", err, &diagnostic)
	}
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
	binding := cfg.Clients[configuration.ClientCodex]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = configuration.AuthenticationClientNative
	cfg.Clients[configuration.ClientCodex] = binding
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &refusingSecretStore{}
	runtime.Secrets = store
	command := NewCommand(runtime)

	err = command.RunE(command, helperArgs(t, runtime, configuration.ClientCodex))
	if err == nil || !strings.Contains(err.Error(), "client-owned authentication") {
		t.Fatalf("credential helper error = %v", err)
	}
	var stderr bytes.Buffer
	presentation.RenderError(presentation.New(&stderr, false), err, false)
	if !strings.Contains(stderr.String(), "aigw verify --for codex") {
		t.Fatalf("client-owned recovery action missing: %q", stderr.String())
	}
	if store.getCalls != 0 || store.existsCalls != 0 {
		t.Fatalf("credential helper accessed client-native credentials: get=%d exists=%d", store.getCalls, store.existsCalls)
	}
	if buffer.Len() != 0 {
		t.Fatalf("credential helper wrote stdout: %q", buffer.String())
	}
}
