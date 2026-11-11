package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/domain"
	"aigw-cli/internal/platform"
)

// TestCredentialViewHoldsOnlyTheFrozenFields pins the reader's view of a
// file with every schema 8 field: each provider's ID and endpoints and each
// client's enabled state and authentication, which a newer version, unknown
// fields and malformed models leave as they are.
func TestCredentialViewHoldsOnlyTheFrozenFields(t *testing.T) {
	_, text := schemaEightConfig(t)
	want := domain.CredentialView{
		Providers: []domain.CredentialProvider{
			{ID: "dmxapi", Endpoints: domain.Endpoints{Anthropic: "https://www.dmxapi.cn", Responses: "https://www.dmxapi.cn/v1"}},
			{ID: "aihubmix", Endpoints: domain.Endpoints{Chat: "https://aihubmix.com/v1"}},
		},
		Clients: map[string]domain.CredentialClient{ClientClaude: {}, ClientCodex: {Enabled: true}, ClientHermes: {}},
	}
	newer := strings.NewReplacer(
		"version = 8", "version = 99\nreader = 'unknown'",
		"label = 'AIHubMix'", "label = 2\nregion = 'eu'",
		"protocols = { chat = [] }", "protocols = 'chat'\ncontext = 'large'",
		"endpoints = { chat = 'https://aihubmix.com/v1' }", "endpoints = { chat = 'https://aihubmix.com/v1', messages = 'https://aihubmix.com/messages' }",
		"enabled = true", "enabled = true\ntheme = 'dark'",
	).Replace(text)
	for name, document := range map[string]string{"schema 8": text, "a newer file": newer} {
		path := filepath.Join(t.TempDir(), "aigw.toml")
		if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}
		view, err := NewStore(path, testAdmission()).LoadCredentialView()
		if err != nil || !reflect.DeepEqual(view, want) {
			t.Fatalf("the view of %s = %+v, %v; want %+v", name, view, err, want)
		}
	}
	authenticated := strings.Replace(text, "protocol = 'anthropic'\n", "protocol = 'anthropic'\nauthentication = 'device-code'\n", 1)
	path := filepath.Join(t.TempDir(), "aigw.toml")
	if err := os.WriteFile(path, []byte(authenticated), 0o600); err != nil {
		t.Fatal(err)
	}
	if view, err := NewStore(path, testAdmission()).LoadCredentialView(); err != nil || view.Clients[ClientClaude].Authentication != "device-code" {
		t.Fatalf("an unknown authentication = %+v, %v", view.Clients[ClientClaude], err)
	}
}

// The view reads the file as Load reads it: an absent file is an empty view,
// a file in the way a read failure, and a document that is not TOML, or a
// field of the view with a value of the wrong type, a parse failure.
func TestCredentialViewNamesTheStageThatFailed(t *testing.T) {
	root := t.TempDir()
	if view, err := NewStore(filepath.Join(root, "aigw.toml"), testAdmission()).LoadCredentialView(); err != nil || len(view.Providers) != 0 || len(view.Clients) != 0 {
		t.Fatalf("an absent file = %+v, %v", view, err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(filepath.Join(file, "aigw.toml"), testAdmission()).LoadCredentialView()
	if failure, ok := errors.AsType[*LoadError](err); !ok || failure.Phase != LoadPhaseRead || !errors.Is(err, platform.ErrFileAsFolder) {
		t.Fatalf("a file in the way = %v", err)
	}
	folder := filepath.Join(root, "folder.toml")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(folder, testAdmission()).LoadCredentialView(); err == nil {
		t.Fatal("a folder was read as the configuration")
	}
	for name, document := range map[string]string{
		"not TOML":       "[[providers]\n",
		"a wrong client": "version = 8\n[clients.codex]\nenabled = 'yes'\n",
		"a wrong ID":     "version = 8\n[[providers]]\nid = 7\n",
	} {
		path := filepath.Join(root, strings.ReplaceAll(name, " ", "-")+".toml")
		if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := NewStore(path, testAdmission()).LoadCredentialView()
		if failure, ok := errors.AsType[*LoadError](err); !ok || failure.Phase != LoadPhaseParse {
			t.Errorf("%s = %v", name, err)
		}
	}
}
