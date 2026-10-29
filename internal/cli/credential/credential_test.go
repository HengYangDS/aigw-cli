package credential

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aigw-cli/internal/cli/invocation"
	"aigw-cli/internal/client/claude"
	"aigw-cli/internal/client/codex"
	"aigw-cli/internal/client/hermes"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/configuration"
	"aigw-cli/internal/presentation"
	"aigw-cli/internal/secrets"
	"aigw-cli/internal/secrets/native"
)

func TestCredentialHelperPrintsOnlyTheProjectedAccountToken(t *testing.T) {
	for _, client := range clients.Admission().IDs() {
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
				route := cfg.Routes[client]
				switch change {
				case "model":
					route.Model = "another-model"
				case "label":
					route.Label = "Another label"
				}
				cfg.Routes[client] = route
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

func TestCredentialHelperIgnoresMalformedOptionalComponents(t *testing.T) {
	for _, client := range clients.Admission().IDs() {
		for _, component := range []string{".catalog.toml", ".capabilities.toml"} {
			t.Run(client+component, func(t *testing.T) {
				runtime, buffer := helperRuntime(t, client, true)
				args := helperArgs(t, runtime, client)
				if err := runtime.Secrets.Set("gateway", "original-token"); err != nil {
					t.Fatal(err)
				}
				path := strings.TrimSuffix(runtime.Config.Path(), filepath.Ext(runtime.Config.Path())) + component
				if err := os.WriteFile(path, []byte("[invalid component"), 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := runtime.Config.Load(); err == nil {
					t.Fatal("invalid optional component was accepted by the full configuration reader")
				}
				command := NewCommand(runtime)
				if err := command.RunE(command, args); err != nil {
					t.Fatal(err)
				}
				if got := buffer.String(); got != "original-token\n" {
					t.Fatalf("credential output = %q, want the projected Account Token", got)
				}
			})
		}
	}
}

func TestCredentialHelperNamesReaderAndStage(t *testing.T) {
	for _, stage := range []string{"read", "parse", "validate"} {
		t.Run(stage, func(t *testing.T) {
			runtime, stdout := helperRuntime(t, codex.ID, true)
			runtime.Version = "9.8.7"
			args := helperArgs(t, runtime, codex.ID)
			path := runtime.Config.Path()
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"AIGW 9.8.7 credential reader", "configuration " + stage + " stage", "aigw sync", "reload the client"}
			switch stage {
			case "read":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				_, readErr := os.ReadFile(path)
				failure, ok := errors.AsType[*os.PathError](readErr)
				if !ok {
					t.Fatalf("directory read error = %v, want a path error", readErr)
				}
				want = []string{"AIGW 9.8.7 credential reader", "configuration read stage", "(" + failure.Op + ": " + failure.Err.Error() + ")", "aigw doctor"}
			case "parse":
				data = []byte("[invalid configuration")
			case "validate":
				data = regexp.MustCompile(`(?m)^openai_responses = \[\]$`).ReplaceAll(data, []byte("openai_responses = ['future_capability']"))
				if !bytes.Contains(data, []byte("future_capability")) {
					t.Fatalf("fixture has no Responses interface to extend:\n%s", data)
				}
			}
			if stage != "read" {
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			command := NewCommand(runtime)
			err = command.RunE(command, args)
			var stderr bytes.Buffer
			presentation.RenderCredentialError(presentation.New(&stderr, false), err)
			diagnostic := stderr.String()
			if err == nil || stdout.Len() != 0 {
				t.Fatalf("error=%v stdout=%q", err, stdout.String())
			}
			for _, text := range want {
				if !strings.Contains(diagnostic, text) {
					t.Fatalf("diagnostic lacks %q:\n%s", text, diagnostic)
				}
			}
			if strings.Contains(diagnostic, filepath.Dir(path)) || strings.Contains(diagnostic, "future_capability") {
				t.Fatalf("diagnostic disclosed a path or configuration content:\n%s", diagnostic)
			}
		})
	}
}

func TestCredentialHelperResolvesAnUnselectedRouteProjection(t *testing.T) {
	for _, client := range clients.Admission().IDs() {
		t.Run(client, func(t *testing.T) {
			runtime, buffer := helperRuntime(t, client, true)
			cfg, err := runtime.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			account := configuration.Account{Label: "Alternate"}
			spec, _ := clients.Admission().Spec(client)
			if spec.EndpointProtocols[0] == configuration.ProtocolAnthropic {
				account.Endpoints.Anthropic = "https://alternate.test"
			} else {
				account.Endpoints.OpenAIResponses = "https://alternate.test/v1"
			}
			cfg.Accounts["alternate"] = account
			cfg.Routes["alternate"] = configuration.Route{
				Label: "Alternate", Account: "alternate", Model: client + "-alternate",
				Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{spec.EndpointProtocols[0]: {}},
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
				t.Fatalf("unselected Route credential = %q", got)
			}
		})
	}
}

func TestCredentialHelperResolvesEveryProjectedRouteProtocol(t *testing.T) {
	runtime, buffer := helperRuntime(t, hermes.ID, true)
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	account := cfg.Accounts["gateway"]
	account.Endpoints.OpenAIChatCompletions = "https://gateway.test/chat/v1"
	cfg.Accounts["gateway"] = account
	route := cfg.Routes[hermes.ID]
	route.Interfaces = map[configuration.EndpointProtocol][]configuration.Capability{
		configuration.ProtocolOpenAIResponses:       {},
		configuration.ProtocolOpenAIChatCompletions: {},
	}
	cfg.Routes[hermes.ID] = route
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Secrets.Set("gateway", "gateway-token"); err != nil {
		t.Fatal(err)
	}
	projection, err := cfg.ResolveRuntime(hermes.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	projection.Protocol = configuration.ProtocolOpenAIChatCompletions
	projection.Endpoint = account.Endpoints.OpenAIChatCompletions

	command := NewCommand(runtime)
	if err := command.RunE(command, []string{hermes.ID, projection.CredentialProjectionFingerprint(hermes.ID)}); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "gateway-token\n" {
		t.Fatalf("alternate protocol credential = %q", got)
	}
}

func TestCredentialHelperRejectsStaleBindingBeforeSecretAccess(t *testing.T) {
	for _, client := range clients.Admission().IDs() {
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
					route := cfg.Routes[client]
					route.Account = "other"
					cfg.Routes[client] = route
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
			runtime, buffer := helperRuntime(t, claude.ID, true)
			args := helperArgs(t, runtime, claude.ID)
			switch name {
			case "load":
				runtime.Config = configuration.NewStore(t.TempDir(), clients.Admission())
			case "disabled":
				runtime, buffer = helperRuntime(t, claude.ID, false)
			case "route":
				cfg, err := runtime.Config.Load()
				if err != nil {
					t.Fatal(err)
				}
				route := cfg.Routes[claude.ID]
				route.Model = "changed-after-projection"
				cfg.Routes[claude.ID] = route
				if err := runtime.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
			}
			argument := claude.ID
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
	runtime, buffer := helperRuntime(t, codex.ID, true)
	store := &refusingSecretStore{}
	runtime.Secrets = store
	command := NewCommand(runtime)
	command.SetArgs([]string{codex.ID})
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
	runtime, stdout := helperRuntime(t, claude.ID, true)
	command := NewCommand(runtime)

	if command.Short != "Read the Account Token matching a client projection" {
		t.Fatalf("short help = %q", command.Short)
	}
	err := command.RunE(command, helperArgs(t, runtime, claude.ID))
	var stderr bytes.Buffer
	presentation.RenderCredentialError(presentation.New(&stderr, false), err)
	if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "claude Account Token is not configured") ||
		!strings.Contains(stderr.String(), "aigw doctor") || strings.Contains(stderr.String(), "gateway") {
		t.Fatalf("credential error = %v, output = %q", err, stderr.String())
	}
}

func TestClaudeCredentialHelperPropagatesOutputFailure(t *testing.T) {
	runtime, _ := helperRuntime(t, claude.ID, true)
	if err := runtime.Secrets.Set("gateway", "secret-token"); err != nil {
		t.Fatal(err)
	}
	writeErr := errors.New("write failed")
	runtime.Out = failingWriter{err: writeErr}
	command := NewCommand(runtime)
	if err := command.RunE(command, helperArgs(t, runtime, claude.ID)); !errors.Is(err, writeErr) {
		t.Fatalf("error=%v", err)
	}
}

func TestCredentialHelperRedactsBackendDiagnostics(t *testing.T) {
	runtime, stdout := helperRuntime(t, codex.ID, true)
	store := &refusingSecretStore{readError: errors.New("locked login Keychain at /private/account-token")}
	runtime.Secrets = store
	command := NewCommand(runtime)
	err := command.RunE(command, helperArgs(t, runtime, codex.ID))
	var stderr bytes.Buffer
	renderer := presentation.New(&stderr, false)
	presentation.RenderCredentialError(renderer, err)
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Account Token could not be read") ||
		!strings.Contains(stderr.String(), "unlock the native store if locked") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if store.getCalls != 1 || store.existsCalls != 0 || strings.Contains(stderr.String(), "/private/account-token") {
		t.Fatalf("raw backend error escaped the credential boundary: %q", stderr.String())
	}
}

func helperRuntime(t *testing.T, client string, enabled bool) (invocation.Context, *bytes.Buffer) {
	t.Helper()
	store := configuration.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := configuration.NewConfig(clients.Admission())
	account := configuration.Account{Label: "Gateway"}
	spec, _ := clients.Admission().Spec(client)
	if spec.EndpointProtocols[0] == configuration.ProtocolAnthropic {
		account.Endpoints.Anthropic = "https://gateway.test"
	} else {
		account.Endpoints.OpenAIResponses = "https://gateway.test/v1"
	}
	cfg.Accounts["gateway"] = account
	cfg.Routes[client] = configuration.Route{
		Label: client, Account: "gateway", Model: client + "-team",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{spec.EndpointProtocols[0]: {}},
	}
	cfg.Clients[client] = configuration.ClientBinding{Route: client, Enabled: enabled, Protocol: spec.EndpointProtocols[0], Executable: client}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return invocation.Context{Clients: clients.Registry(), Config: store, Secrets: secrets.NewMemoryStore(), Out: out}, out
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
	runtime, stdout := helperRuntime(t, codex.ID, true)
	runtime.Secrets = &refusingSecretStore{readError: context.DeadlineExceeded}
	command := NewCommand(runtime)
	err := command.RunE(command, helperArgs(t, runtime, codex.ID))
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
	runtime, buffer := helperRuntime(t, codex.ID, true)
	cfg, err := runtime.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[codex.ID]
	binding.ModelProvider = "amazon-bedrock"
	binding.Authentication = configuration.AuthenticationClientNative
	cfg.Clients[codex.ID] = binding
	if err := runtime.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	store := &refusingSecretStore{}
	runtime.Secrets = store
	command := NewCommand(runtime)

	err = command.RunE(command, helperArgs(t, runtime, codex.ID))
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

func TestCredentialHelperReportsSafeNativeStatusWithoutRetry(t *testing.T) {
	runtime, output := helperRuntime(t, codex.ID, true)
	store := &refusingSecretStore{readError: &native.KeychainError{Status: -25308}}
	runtime.Secrets = store
	command := NewCommand(runtime)
	err := command.RunE(command, helperArgs(t, runtime, codex.ID))
	var diagnostic bytes.Buffer
	presentation.RenderCredentialError(presentation.New(&diagnostic, false), err)
	if output.Len() != 0 || !strings.Contains(diagnostic.String(), "Keychain OSStatus -25308") ||
		!strings.Contains(diagnostic.String(), "repeated helper calls cannot grant access") || store.getCalls != 1 || store.existsCalls != 0 {
		t.Fatalf("native credential result = %q, calls=%d/%d", diagnostic.String(), store.getCalls, store.existsCalls)
	}
	failure, ok := errors.AsType[*native.KeychainError](err)
	if !ok || failure.Status != -25308 {
		t.Fatal("CLI wrapper lost typed native status")
	}
}
