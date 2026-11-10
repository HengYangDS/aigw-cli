package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

func TestUsePreviewDoesNotAcquireConfigurationLock(t *testing.T) {
	app := configuredApp(t)
	var output bytes.Buffer
	app.Out, app.Err, app.Secrets = &output, &output, secret.NewMemoryStore()
	app.Discovery = noClientDiscovery{}
	before, err := app.Config.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(app.Config.Path()+".lock", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Execute(app, []string{"use", "codex", "one/gpt", "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	after, err := app.Config.CaptureSnapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("read-only use changed configuration or recovery files: %v", err)
	}
	if !strings.Contains(output.String(), `"dry_run": true`) {
		t.Fatalf("selection preview omitted its structured result: %s", output.String())
	}
}

func TestInvalidMutationArgumentsLeaveConfigurationStorageAbsent(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
		next string
	}{
		{[]string{"use"}, "aigw use without a terminal needs a <client> and a <provider/model> argument", "aigw use --help"},
		{[]string{"use", "codex"}, "aigw use without a terminal needs a <provider/model> argument after the client", "aigw use --help"},
		{[]string{"use", "codx", "one/gpt"}, `unknown client "codx"; did you mean codex?`, "aigw client list"},
		{[]string{"setup", "--from", "missing-manifest.toml", "--token-stdin"}, "--token-stdin reads one Token, so it needs exactly one --provider", "aigw setup --help"},
		{[]string{"client", "enable", "unknown"}, `unknown client "unknown"`, "aigw client list"},
		{[]string{"client", "disable", "unknown"}, `unknown client "unknown"`, "aigw client list"},
		{[]string{"client", "edit", "codx", "--direct"}, "did you mean codex?", "aigw client list"},
		{[]string{"client", "edit", "codex"}, "client edit needs --executable", "aigw client edit --help"},
		{[]string{"client", "edit"}, "needs a <client> argument", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--executable", " "}, "--executable requires a non-empty path", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--target", " "}, "--target requires a non-empty path", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--target", "a.toml", "--target", "a.toml"}, "twice", "aigw client edit --help"},
		{[]string{"client", "edit", "claude", "--target", "settings.json"}, "Claude has no configuration file to name; --target is for Claude Desktop, Codex and Hermes", "aigw client edit --help"},
		{[]string{"client", "enable", "claude", "--executable", "/opt/claude", "--target", "settings.json"}, "Claude has no configuration file to name; --target is for Claude Desktop, Codex and Hermes", "aigw client enable --help"},
		{[]string{"client", "edit", "codex", "--via", " "}, "--via requires an endpoint URL", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--via", "http://proxy.test/v1"}, "plain HTTP is allowed only for a loopback endpoint", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--via", "http://127.0.0.1:8792/v1", "--direct"}, "mutually exclusive", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--via", "http://127.0.0.1:8792/v1", "--via", "http://127.0.0.1:8793/v1"}, "this flag takes one value", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--request-retries", "1", "--request-retries", "2"}, "this flag takes one value", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--stream-retries", "-1"}, "expected a whole number of retries", "aigw client edit --help"},
		{[]string{"client", "edit", "codex", "--native-retries", "--stream-retries", "0"}, "mutually exclusive", "aigw client edit --help"},
		{[]string{"client", "edit", "hermes", "--request-retries", "0"}, "Hermes has no native retry budgets", "aigw client edit --help"},
		{[]string{"client", "edit", "claude", "--native-retries"}, "Claude has no native retry budgets", "aigw client edit --help"},
		{[]string{"client", "enable", "claude"}, "--executable is required", "aigw client list"},
		{[]string{"client", "enable", "claude", "--executable", " "}, "--executable is required", "aigw client list"},
		{[]string{"client", "enable", "codex", "--executable", "codex"}, "requires at least one --target", "aigw client enable --help"},
		{[]string{"client", "enable", "codex", "--executable", "codex", "--target", " "}, "--target requires a non-empty path", "aigw client enable --help"},
		{[]string{"config", "import", " "}, "manifest path must not be blank", "aigw config import --help"},
	} {
		t.Run(strings.Join(test.args, "/"), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "configuration")
			var output bytes.Buffer
			app := &App{Clients: clients.Registry(),
				Config: store.NewStore(filepath.Join(root, "config.toml"), clients.Admission()),
				Out:    &output,
				Err:    io.Discard,
			}
			if err := Execute(app, test.args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("argument error = %v, want %q", err, test.want)
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("argument rejection created configuration storage: %v", err)
			}
			if !strings.Contains(output.String(), test.next) {
				t.Fatalf("argument rejection omitted its repair action %q: %s", test.next, &output)
			}
		})
	}
}

func TestConfigurationLockUsesParsedOperations(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "provider add", args: []string{"provider", "add", "team"}, want: true},
		{name: "provider add dry-run", args: []string{"provider", "add", "team", "--dry-run"}, want: false},
		{name: "provider edit", args: []string{"provider", "edit", "team"}, want: true},
		{name: "provider edit dry-run", args: []string{"provider", "edit", "team", "--dry-run"}, want: false},
		{name: "provider remove", args: []string{"provider", "remove", "team"}, want: true},
		{name: "provider remove dry-run", args: []string{"provider", "remove", "team", "--dry-run"}, want: false},
		{name: "provider rotate", args: []string{"provider", "rotate", "team"}, want: true},
		{name: "provider list", args: []string{"provider", "list"}, want: false},
		{name: "provider show", args: []string{"provider", "show", "team"}, want: false},
		{name: "provider balance", args: []string{"provider", "balance"}, want: false},
		{name: "model add", args: []string{"model", "add", "team/new"}, want: true},
		{name: "model add dry-run", args: []string{"model", "add", "team/new", "--dry-run"}, want: false},
		{name: "model edit", args: []string{"model", "edit", "team/model"}, want: true},
		{name: "model edit dry-run", args: []string{"model", "edit", "team/model", "--dry-run"}, want: false},
		{name: "model remove", args: []string{"model", "remove", "team/model"}, want: true},
		{name: "model remove dry-run", args: []string{"model", "remove", "team/model", "--dry-run"}, want: false},
		{name: "model remove dry-run false", args: []string{"model", "remove", "team/model", "--dry-run=false"}, want: true},
		{name: "model list", args: []string{"model", "list"}, want: false},
		{name: "model show", args: []string{"model", "show", "team/model"}, want: false},
		{name: "model search", args: []string{"model", "search"}, want: false},
		{name: "bare model", args: []string{"model"}, want: false},
		{name: "bare provider", args: []string{"provider"}, want: false},
		{name: "sync apply", args: []string{"sync"}, want: true},
		{name: "sync dry-run", args: []string{"sync", "--dry-run"}, want: false},
		{name: "sync dry-run equals", args: []string{"sync", "--dry-run=true"}, want: false},
		{name: "sync dry-run false", args: []string{"sync", "--dry-run=false"}, want: true},
		{name: "sync final dry-run false", args: []string{"sync", "--dry-run", "--dry-run=false"}, want: true},
		{name: "sync final dry-run true", args: []string{"sync", "--dry-run=false", "--dry-run"}, want: false},
		{name: "setup", args: []string{"setup", "--route", "route"}, want: true},
		{name: "setup dry-run", args: []string{"setup", "--route", "route", "--dry-run"}, want: false},
		{name: "client enable dry-run", args: []string{"client", "enable", "codex", "--dry-run"}, want: false},
		{name: "client disable dry-run", args: []string{"client", "disable", "codex", "--dry-run"}, want: false},
		{name: "bare client", args: []string{"client"}, want: false},
		{name: "client enable", args: []string{"client", "enable", "codex"}, want: true},
		{name: "client disable", args: []string{"client", "disable", "codex"}, want: true},
		{name: "client edit", args: []string{"client", "edit", "codex", "--direct"}, want: true},
		{name: "client edit dry-run", args: []string{"client", "edit", "codex", "--direct", "--dry-run"}, want: false},
		{name: "client list", args: []string{"client", "list"}, want: false},
		{name: "bare config", args: []string{"config"}, want: false},
		{name: "config import", args: []string{"config", "import", "path"}, want: true},
		{name: "config import dry-run", args: []string{"config", "import", "path", "--dry-run"}, want: false},
		{name: "config import dry-run false", args: []string{"config", "import", "path", "--dry-run=false"}, want: true},
		{name: "config import final dry-run false", args: []string{"config", "import", "path", "--dry-run", "--dry-run=false"}, want: true},
		{name: "config import final dry-run true", args: []string{"config", "import", "path", "--dry-run=false", "--dry-run"}, want: false},
		{name: "config migrate", args: []string{"config", "migrate"}, want: true},
		{name: "config migrate dry-run", args: []string{"config", "migrate", "--dry-run"}, want: false},
		{name: "config migration rollback", args: []string{"config", "migrate", "--rollback"}, want: true},
		{name: "config migration rollback preview", args: []string{"config", "migrate", "--rollback", "--dry-run"}, want: false},
		{name: "config export", args: []string{"config", "export"}, want: false},
		{name: "config rollback", args: []string{"config", "rollback"}, want: true},
		{name: "config rollback dry-run", args: []string{"config", "rollback", "--dry-run"}, want: false},
		{name: "self update", args: []string{"self", "update"}, want: true},
		{name: "self update dry-run", args: []string{"self", "update", "--dry-run"}, want: false},
		{name: "self rollback", args: []string{"self", "rollback"}, want: true},
		{name: "self rollback dry-run", args: []string{"self", "rollback", "--dry-run"}, want: false},
		{name: "self uninstall", args: []string{"self", "uninstall"}, want: true},
		{name: "self uninstall dry-run", args: []string{"self", "uninstall", "--dry-run"}, want: false},
		{name: "self install", args: []string{"self", "install"}, want: false},
		{name: "self status", args: []string{"self", "status"}, want: false},
		{name: "bare aigw", args: []string{}, want: false},
		{name: "status", args: []string{"status"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{Clients: clients.Registry()}
			command, args, err := NewRoot(app).Find(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if err := command.ParseFlags(args); err != nil {
				t.Fatal(err)
			}
			if got := requiresConfigurationLock(command); got != tt.want {
				t.Fatalf("requiresConfigurationLock(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestConfigImportPreviewDoesNotCreateConfigurationDirectoryOrLock(t *testing.T) {
	root := filepath.Join(t.TempDir(), "configuration")
	var output bytes.Buffer
	app := &App{Clients: clients.Registry(), Config: store.NewStore(filepath.Join(root, "config.toml"), clients.Admission()), Out: &output, Err: io.Discard}
	manifest := filepath.Join("..", "..", "manifests", "team.toml")
	if err := Execute(app, []string{"config", "import", manifest, "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only import created configuration storage: %v", err)
	}
	if !strings.Contains(output.String(), `"dry_run": true`) {
		t.Fatalf("import preview omitted structured result: %q", output.String())
	}
}

// TestBareAigwTakesNoConfigurationLock: bare aigw is aigw status, which writes
// nothing, in an empty home at a terminal, without one, and in a configured
// home.
func TestBareAigwTakesNoConfigurationLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuration.toml")
	configStore := store.NewStore(path, clients.Admission())
	cfg := domain.NewConfig(clients.Admission())
	cfg.Providers["dmx"] = domain.Provider{Label: "DMX", Endpoints: domain.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Models["gpt"] = domain.Model{
		Label: "GPT", Provider: "dmx", Family: "gpt-test",
		Protocols: map[domain.EndpointProtocol][]domain.Capability{domain.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedModel(codex.ID, "gpt", "")
	if err := configStore.Save(cfg); err != nil {
		t.Fatal(err)
	}
	for name, app := range map[string]App{
		"an empty home at a terminal":     {Config: store.NewStore(filepath.Join(t.TempDir(), "missing.toml"), clients.Admission()), Interactive: true},
		"an empty home without one":       {Config: store.NewStore(filepath.Join(t.TempDir(), "missing.toml"), clients.Admission())},
		"a configured home at a terminal": {Config: configStore, Interactive: true},
	} {
		if requiresConfigurationLock(NewRoot(&app)) {
			t.Fatalf("bare aigw in %s takes the configuration lock", name)
		}
	}
}
