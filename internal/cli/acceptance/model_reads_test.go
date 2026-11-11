package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// The model commands own what `aigw route` covered beyond their own tests: a
// model read never touches a credential, and a model edit reaches the files
// of the client that uses the model.

// A model read observes no credential: `aigw model list` and `aigw model
// show`, in text and JSON, never ask the credential backend, name no client's
// authentication, which belongs to the client, and write nothing.
func TestModelReadsNeitherObserveCredentialsNorShowClientAuthentication(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["native"] = domain.Provider{Label: "Native", Endpoints: domain.Endpoints{Responses: "https://native.test/v1"}}
	cfg.Models["native"] = domain.Model{Label: "Native", Provider: "native", Family: "native-model",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolResponses: {}}}
	cfg.Clients[codex.ID] = domain.ClientBinding{Model: "native", ModelProvider: "amazon-bedrock", Authentication: domain.AuthenticationClientNative}
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("native", "model-read-token-must-not-appear"); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, app.Config.Path())
	observed := &recordingCredentialStore[string]{backend: secretStore, existsErr: errors.New("a model read observed a credential")}
	app.Secrets = observed
	for _, args := range [][]string{{"model", "list"}, {"model", "list", "--json"}, {"model", "show", "native/native-model"}, {"model", "show", "native/native-model", "--json"}} {
		out.Reset()
		if err := cli.Execute(app, args); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		text := out.String()
		if strings.Contains(text, "model-read-token-must-not-appear") || strings.Contains(text, "Authentication") || strings.Contains(text, "amazon-bedrock") {
			t.Fatalf("%v shows a Token or the client's authentication:\n%s", args, text)
		}
		for _, field := range []string{"authentication", "model_provider", "secret_available", "credential_ownership"} {
			if strings.Contains(text, `"`+field+`"`) {
				t.Fatalf("%v holds the client field %q:\n%s", args, field, text)
			}
		}
	}
	if len(observed.existsCalls)+len(observed.getCalls) != 0 {
		t.Fatalf("model reads observed credentials: exists=%q get=%q", observed.existsCalls, observed.getCalls)
	}
	if after := readFile(t, app.Config.Path()); !bytes.Equal(after, before) {
		t.Fatal("a model read changed the configuration")
	}
}

// An edit of the model that an enabled Codex uses writes Codex's files.
func TestModelEditReachesTheFilesOfTheClientThatUsesIt(t *testing.T) {
	app, out, secretStore, _, _ := testApp(t, "")
	target := filepath.Join(t.TempDir(), "configuration.toml")
	if err := os.WriteFile(target, []byte("model_provider = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "one", "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, codex.ID, "gpt")
	cfg.SetSelectedModel(codex.ID, "one", "")
	cfg.SetClientActivation(codex.ID, true, "/opt/codex", []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("one", "token"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Execute(app, []string{"model", "edit", "one/gpt", "--label", "New Label"}); err != nil {
		t.Fatalf("model edit: %v\n%s", err, out)
	}
	if projected, err := os.ReadFile(target); err != nil || !strings.Contains(string(projected), "AIGW: New Label") {
		t.Fatalf("the label did not reach Codex's file: %s, %v", projected, err)
	}
}
