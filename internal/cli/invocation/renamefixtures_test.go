package invocation

import (
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
	"aigw-cli/internal/terminal"
	"bytes"
	"path/filepath"
	"testing"
)

func renameRuntime(t *testing.T) Context {
	t.Helper()
	configStore := store.NewStore(filepath.Join(t.TempDir(), "configuration.toml"), clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Accounts["old"] = domain.Account{Label: "Old", Endpoints: domain.Endpoints{OpenAIResponses: "https://old.test/v1"}}
	cfg.Routes["old"] = domain.Route{Label: "Old", Account: "old", Model: "gpt"}
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	secretStore := secret.NewMemoryStore()
	diagnostics, err := secret.NewDiagnosticCredentialStore(secretStore)
	if err != nil {
		t.Fatal(err)
	}
	return Context{Clients: clients.Registry(), Config: configStore, Secrets: secretStore, Accounts: diagnostics, Out: &bytes.Buffer{}}
}

type selectionPrompt struct {
	selected  string
	selectErr error
	text      string
	textErr   error
}

func (prompt selectionPrompt) Secret(string) (string, error) { return "", nil }

func (prompt selectionPrompt) Select(string, []terminal.Choice) (string, error) {
	return prompt.selected, prompt.selectErr
}

func (prompt selectionPrompt) Text(string) (string, error) { return prompt.text, prompt.textErr }
