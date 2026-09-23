package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "aigw-cli/internal/configuration"
)

func TestInvalidMutationArgumentsLeaveConfigurationStorageAbsent(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
		next string
	}{
		{[]string{"use"}, "requires a Route", "aigw use --for <client> <route>"},
		{[]string{"use", "route"}, "requires --for", "aigw use --for <client> <route>"},
		{[]string{"setup", "--from", "missing-manifest.toml", "--token-stdin"}, "--token-stdin requires --account", "aigw setup --help"},
		{[]string{"account", "rename", "old", "--finalize"}, "requires explicit <old> <new>", "aigw account rename --help"},
		{[]string{"account", "rename"}, "requires <old> <new>", "aigw account rename --help"},
		{[]string{"account", "rename", "old"}, "requires <old> <new>", "aigw account rename --help"},
		{[]string{"route", "rename"}, "requires <old> <new>", "aigw route rename --help"},
		{[]string{"route", "rename", "old"}, "requires <old> <new>", "aigw route rename --help"},
		{[]string{"account", "rename", "", "new"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "bad id"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"route", "rename", "bad id", "new"}, "Invalid route ID", "aigw route rename --help"},
		{[]string{"route", "rename", "old", ""}, "Invalid route ID", "aigw route rename --help"},
		{[]string{"account", "rename", "bad id", "new", "--finalize"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "bad id", "--finalize", "--dry-run"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "new", "--confirm-api-token-rotation"}, "confirmations require --finalize", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "new", "--confirm-account-probe-rotation"}, "confirmations require --finalize", "aigw account rename --help"},
		{[]string{"client", "enable", "unknown"}, "invalid argument", "aigw client enable --help"},
		{[]string{"client", "disable", "unknown"}, "invalid argument", "aigw client disable --help"},
		{[]string{"client", "enable", "claude"}, "--executable is required", "aigw client discover"},
		{[]string{"client", "enable", "claude", "--executable", " "}, "--executable is required", "aigw client discover"},
		{[]string{"client", "enable", "codex", "--executable", "codex"}, "requires at least one --target", "aigw client enable --help"},
		{[]string{"client", "enable", "codex", "--executable", "codex", "--target", " "}, "--target requires a non-empty path", "aigw client enable --help"},
		{[]string{"add", "new"}, "--for and --model are required", "aigw add --help"},
		{[]string{"add", "bad id"}, "Invalid account ID", "aigw add --help"},
		{[]string{"add", "new", "--for", "unknown", "--model", "model"}, "--for and --model are required", "aigw add --help"},
		{[]string{"add", "new", "--for", "codex", "--model", " "}, "--for and --model are required", "aigw add --help"},
		{[]string{"route", "add", "new"}, "--account, --model, and --protocol are required", "aigw route add --help"},
		{[]string{"route", "add", "bad id"}, "Invalid route ID", "aigw route add --help"},
		{[]string{"route", "add", "new", "--account", "bad id", "--model", "model"}, "Invalid account ID", "aigw route add --help"},
		{[]string{"route", "add", "new", "--account", "account", "--model", " "}, "--account, --model, and --protocol are required", "aigw route add --help"},
		{[]string{"config", "import", " "}, "manifest path must not be blank", "aigw config import --help"},
		{[]string{"account", "edit", "account"}, "at least one of the flags", "aigw account edit --help"},
		{[]string{"account", "edit", "bad id", "--label", "Name"}, "Invalid account ID", "aigw account edit --help"},
		{[]string{"account", "edit", "account", "--label", " "}, "--label requires a non-empty value", "aigw account edit --help"},
		{[]string{"account", "edit", "account", "--openai-url", ""}, "--openai-url requires a non-empty value", "aigw account edit --help"},
		{[]string{"account", "edit", "account", "--anthropic-url", " "}, "--anthropic-url requires a non-empty value", "aigw account edit --help"},
		{[]string{"route", "edit", "route"}, "at least one of the flags", "aigw route edit --help"},
		{[]string{"route", "edit", "bad id", "--label", "Name"}, "Invalid route ID", "aigw route edit --help"},
		{[]string{"route", "edit", "route", "--label", " "}, "--label requires a non-empty value", "aigw route edit --help"},
		{[]string{"route", "remove", " "}, "Invalid route ID", "aigw route remove --help"},
		{[]string{"account", "diagnostics", "enable", "account"}, "requires an interactive terminal", "aigw account diagnostics enable --help"},
	} {
		t.Run(strings.Join(test.args, "/"), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "configuration")
			var output bytes.Buffer
			app := &App{
				Config: configuration.NewStore(filepath.Join(root, "config.toml")),
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
		{name: "top-level add", args: []string{"add", "account"}, want: true},
		{name: "account edit", args: []string{"account", "edit", "account"}, want: true},
		{name: "account diagnostics enable", args: []string{"account", "diagnostics", "enable", "account"}, want: true},
		{name: "account rename", args: []string{"account", "rename", "old", "new"}, want: true},
		{name: "account rename dry-run", args: []string{"account", "rename", "old", "new", "--dry-run"}, want: false},
		{name: "account rename dry-run equals", args: []string{"account", "rename", "old", "new", "--dry-run=true"}, want: false},
		{name: "account rename dry-run false", args: []string{"account", "rename", "old", "new", "--dry-run=false"}, want: true},
		{name: "account rename finalize", args: []string{"account", "rename", "old", "new", "--finalize"}, want: true},
		{name: "account rename finalize dry-run", args: []string{"account", "rename", "old", "new", "--finalize", "--dry-run"}, want: false},
		{name: "route add", args: []string{"route", "add", "route"}, want: true},
		{name: "route edit", args: []string{"route", "edit", "route"}, want: true},
		{name: "route rename", args: []string{"route", "rename", "old", "new"}, want: true},
		{name: "route rename dry-run", args: []string{"route", "rename", "old", "new", "--dry-run"}, want: false},
		{name: "route rename dry-run equals", args: []string{"route", "rename", "old", "new", "--dry-run=true"}, want: false},
		{name: "route rename dry-run false", args: []string{"route", "rename", "old", "new", "--dry-run=false"}, want: true},
		{name: "route remove", args: []string{"route", "remove", "route"}, want: true},
		{name: "route list", args: []string{"route", "list"}, want: false},
		{name: "route show", args: []string{"route", "show", "route"}, want: false},
		{name: "account list", args: []string{"account", "list"}, want: false},
		{name: "repair apply", args: []string{"repair"}, want: true},
		{name: "repair dry-run", args: []string{"repair", "--dry-run"}, want: false},
		{name: "repair dry-run equals", args: []string{"repair", "--dry-run=true"}, want: false},
		{name: "repair dry-run false", args: []string{"repair", "--dry-run=false"}, want: true},
		{name: "sync apply", args: []string{"sync"}, want: true},
		{name: "sync dry-run", args: []string{"sync", "--dry-run"}, want: false},
		{name: "sync dry-run equals", args: []string{"sync", "--dry-run=true"}, want: false},
		{name: "sync dry-run false", args: []string{"sync", "--dry-run=false"}, want: true},
		{name: "sync final dry-run false", args: []string{"sync", "--dry-run", "--dry-run=false"}, want: true},
		{name: "sync final dry-run true", args: []string{"sync", "--dry-run=false", "--dry-run"}, want: false},
		{name: "update", args: []string{"update"}, want: true},
		{name: "uninstall", args: []string{"uninstall"}, want: true},
		{name: "bare account", args: []string{"account"}, want: false},
		{name: "bare route", args: []string{"route"}, want: false},
		{name: "bare client", args: []string{"client"}, want: false},
		{name: "client enable", args: []string{"client", "enable", "codex"}, want: true},
		{name: "client disable", args: []string{"client", "disable", "codex"}, want: true},
		{name: "bare config", args: []string{"config"}, want: false},
		{name: "config import", args: []string{"config", "import", "path"}, want: true},
		{name: "config migrate", args: []string{"config", "migrate"}, want: true},
		{name: "config migrate dry-run", args: []string{"config", "migrate", "--dry-run"}, want: false},
		{name: "config migration rollback", args: []string{"config", "migrate", "--rollback"}, want: true},
		{name: "config migration rollback preview", args: []string{"config", "migrate", "--rollback", "--dry-run"}, want: false},
		{name: "config export", args: []string{"config", "export"}, want: false},
		{name: "status", args: []string{"status"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{}
			command, args, err := NewRoot(app).Find(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if err := command.ParseFlags(args); err != nil {
				t.Fatal(err)
			}
			if got := requiresConfigurationLock(app, command); got != tt.want {
				t.Fatalf("requiresConfigurationLock(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestConfigurationLockForInteractiveOnboarding(t *testing.T) {
	emptyStore := App{Config: configuration.NewStore(filepath.Join(t.TempDir(), "missing.toml")), Interactive: true}
	if !requiresConfigurationLock(&emptyStore, NewRoot(&emptyStore)) {
		t.Fatal("an interactive terminal with no routes should trigger the onboarding wizard lock")
	}

	nonInteractive := App{Config: configuration.NewStore(filepath.Join(t.TempDir(), "missing.toml")), Interactive: false}
	if requiresConfigurationLock(&nonInteractive, NewRoot(&nonInteractive)) {
		t.Fatal("a non-interactive session with no routes must not take a mutation lock")
	}

	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Routes["gpt"] = configuration.Route{
		Label: "GPT", Account: "dmx", Model: "gpt-test",
		Interfaces: map[configuration.EndpointProtocol][]configuration.Capability{configuration.ProtocolOpenAIResponses: {}},
	}
	cfg.SetSelectedRoute(configuration.ClientCodex, "gpt")
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	populated := App{Config: store, Interactive: true}
	if requiresConfigurationLock(&populated, NewRoot(&populated)) {
		t.Fatal("an already-configured store must not trigger the onboarding wizard lock")
	}
}
