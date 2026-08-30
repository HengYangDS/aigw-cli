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
				cfg.Profiles["codex"] = configuration.Profile{Label: "Codex", Account: "gateway", Client: configuration.ClientCodex, Models: configuration.Models{configuration.ClientCodex: "gpt"}}
				cfg.Routes.Default = "codex"
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
	cfg.Profiles[client] = configuration.Profile{Label: client, Account: "gateway", Client: client, Models: configuration.Models{client: client + "-team"}}
	cfg.Routes.Default = client
	cfg.Adapters[client] = configuration.AdapterConfig{Enabled: enabled, Executable: client}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return invocation.Context{Config: store, Secrets: secrets.NewMemoryStore(), Out: &bytes.Buffer{}}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
