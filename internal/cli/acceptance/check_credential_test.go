package cli_test

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/secret/native"
)

func TestCheckDoesNotRecommendRotationForUnreadableToken(t *testing.T) {
	app, out, secretStore, _, httpClient := testApp(t, "")
	saveCommandModel(t, app, domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test")
	cfg, err := app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := secretStore.Set("one", "fixture-token"); err != nil {
		t.Fatal(err)
	}
	want := fmt.Errorf("private credential detail: %w", native.ErrUnavailable)
	app.Secrets = &recordingCredentialStore[string]{backend: secretStore, getErr: want}

	err = cli.Execute(app, []string{"check", "claude"})
	if !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("check error = %v, want unreadable credential", err)
	}
	for _, text := range []string{"The credential backend could not return provider one's Token", "Fix: Make the credential store available to this session"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("check output lacks %q: %s", text, out.String())
		}
	}
	for _, forbidden := range []string{"rotate", "private credential detail", "fixture-token"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("check output contains %q: %s", forbidden, out.String())
		}
	}
	if httpClient.calls != 0 {
		t.Fatalf("unreadable credential sent %d provider requests", httpClient.calls)
	}
}

// The environment backend cannot rotate a Token, so a rejected Token names the
// variable that supplies it, in human and JSON output alike.
func TestCheckNamesTheEnvironmentTokenForARejectedToken(t *testing.T) {
	app, out, _, _, httpClient := testApp(t, "")
	cfg := domain.NewConfig(clients.Admission())
	addProviderModel(&cfg, "dmx", "dmx", "DMXAPI", domain.Endpoints{Anthropic: "https://dmx.test"}, claude.ID, "claude-test")
	cfg.SetSelectedModel(claude.ID, "dmx", "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	synchronizeClaudeProjection(t, app, cfg)
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	variable := secret.EnvironmentKey("dmx")
	app.Secrets = secret.NewEnvironmentStore(func(key string) string {
		if key == variable {
			return "rejected-fixture-token"
		}
		return ""
	})
	httpClient.handler = func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"message":"invalid token"}`)), Request: req}, nil
	}
	for _, args := range [][]string{{"check", "--depth", "endpoint"}, {"check", "--depth", "endpoint", "--json"}} {
		out.Reset()
		_ = cli.Execute(app, args)
		if !strings.Contains(out.String(), variable) || strings.Contains(out.String(), "rotate") || strings.Contains(out.String(), "rejected-fixture-token") {
			t.Fatalf("%v recovery does not follow the environment backend:\n%s", args, out.String())
		}
	}
}

// storeBytes is every file below root with its bytes, so that a test can show
// that a command left a file Token store exactly as it was.
func storeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		files[path] = string(data)
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// A one-time Token from standard input authenticates only the check that read
// it: a file Token store holds the same files with the same bytes afterwards,
// whether the provider accepts the Token or rejects it, and the stored Token
// was never replaced by it.
func TestACheckWithAOneTimeTokenLeavesAFileStoreAsItWas(t *testing.T) {
	for name, status := range map[string]int{"accepted": http.StatusOK, "rejected": http.StatusUnauthorized} {
		t.Run(name, func(t *testing.T) {
			client := newBlockedStoreClient(t)
			before := storeBytes(t, client.secrets)
			if len(before) == 0 {
				t.Fatal("the file store holds no Token to compare")
			}
			var tokens []string
			client.app.HTTP = &fakeHTTP{handler: func(req *http.Request) (*http.Response, error) {
				tokens = append(tokens, req.Header.Get("X-Api-Key"))
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"claude-test"}]}`)), Request: req}, nil
			}}
			client.app.In = strings.NewReader("sk-once-0001\n")
			client.output.Reset()
			_ = cli.Execute(client.app, []string{"check", claude.ID, "--model", "gateway/claude-test", "--token-stdin", "--depth", "endpoint", "--json"})
			if len(tokens) == 0 || tokens[0] != "sk-once-0001" {
				t.Fatalf("the check sent %q, not the one-time Token\n%s", tokens, client.output)
			}
			if after := storeBytes(t, client.secrets); !maps.Equal(after, before) {
				t.Fatalf("the check changed the file store:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}
