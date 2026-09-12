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
		{[]string{"use"}, "requires a profile", "aigw use <profile>"},
		{[]string{"setup", "--from", "missing-manifest.toml", "--token-stdin"}, "--token-stdin requires --account", "aigw setup --help"},
		{[]string{"account", "rename", "old", "--finalize"}, "requires explicit <old> <new>", "aigw account rename --help"},
		{[]string{"account", "rename"}, "requires <old> <new>", "aigw account rename --help"},
		{[]string{"account", "rename", "old"}, "requires <old> <new>", "aigw account rename --help"},
		{[]string{"profile", "rename"}, "requires <old> <new>", "aigw profile rename --help"},
		{[]string{"profile", "rename", "old"}, "requires <old> <new>", "aigw profile rename --help"},
		{[]string{"account", "rename", "", "new"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "bad id"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"profile", "rename", "bad id", "new"}, "Invalid profile ID", "aigw profile rename --help"},
		{[]string{"profile", "rename", "old", ""}, "Invalid profile ID", "aigw profile rename --help"},
		{[]string{"account", "rename", "bad id", "new", "--finalize"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "bad id", "--finalize", "--dry-run"}, "Invalid account ID", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "new", "--confirm-api-token-rotation"}, "confirmations require --finalize", "aigw account rename --help"},
		{[]string{"account", "rename", "old", "new", "--confirm-account-probe-rotation"}, "confirmations require --finalize", "aigw account rename --help"},
		{[]string{"adapter", "enable", "unknown"}, "invalid argument", "aigw adapter enable --help"},
		{[]string{"adapter", "disable", "unknown"}, "invalid argument", "aigw adapter disable --help"},
		{[]string{"adapter", "enable", "claude"}, "--executable is required", "aigw adapter discover"},
		{[]string{"adapter", "enable", "claude", "--executable", " "}, "--executable is required", "aigw adapter discover"},
		{[]string{"adapter", "enable", "codex", "--executable", "codex"}, "requires at least one --target", "aigw adapter enable --help"},
		{[]string{"adapter", "enable", "codex", "--executable", "codex", "--target", " "}, "--target requires a non-empty path", "aigw adapter enable --help"},
		{[]string{"add", "new"}, "--for and --model are required", "aigw add --help"},
		{[]string{"add", "bad id"}, "Invalid service ID", "aigw add --help"},
		{[]string{"add", "new", "--for", "unknown", "--model", "model"}, "--for and --model are required", "aigw add --help"},
		{[]string{"add", "new", "--for", "codex", "--model", " "}, "--for and --model are required", "aigw add --help"},
		{[]string{"profile", "add", "new"}, "--account, --for, and --model are required", "aigw profile add --help"},
		{[]string{"profile", "add", "bad id"}, "Invalid profile ID", "aigw profile add --help"},
		{[]string{"profile", "add", "new", "--account", "bad id", "--for", "codex", "--model", "model"}, "Invalid account ID", "aigw profile add --help"},
		{[]string{"profile", "add", "new", "--account", "account", "--for", "unknown", "--model", "model"}, "--for must be", "aigw profile add --help"},
		{[]string{"profile", "add", "new", "--account", "account", "--for", "codex", "--model", " "}, "--account, --for, and --model are required", "aigw profile add --help"},
		{[]string{"config", "import", " "}, "manifest path must not be blank", "aigw config import --help"},
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
		{name: "account connect", args: []string{"account", "connect", "account"}, want: true},
		{name: "account rename", args: []string{"account", "rename", "old", "new"}, want: true},
		{name: "account rename dry-run", args: []string{"account", "rename", "old", "new", "--dry-run"}, want: false},
		{name: "account rename dry-run equals", args: []string{"account", "rename", "old", "new", "--dry-run=true"}, want: false},
		{name: "account rename dry-run false", args: []string{"account", "rename", "old", "new", "--dry-run=false"}, want: true},
		{name: "account rename finalize", args: []string{"account", "rename", "old", "new", "--finalize"}, want: true},
		{name: "account rename finalize dry-run", args: []string{"account", "rename", "old", "new", "--finalize", "--dry-run"}, want: false},
		{name: "profile add", args: []string{"profile", "add", "profile"}, want: true},
		{name: "profile edit", args: []string{"profile", "edit", "profile"}, want: true},
		{name: "profile rename", args: []string{"profile", "rename", "old", "new"}, want: true},
		{name: "profile rename dry-run", args: []string{"profile", "rename", "old", "new", "--dry-run"}, want: false},
		{name: "profile rename dry-run equals", args: []string{"profile", "rename", "old", "new", "--dry-run=true"}, want: false},
		{name: "profile rename dry-run false", args: []string{"profile", "rename", "old", "new", "--dry-run=false"}, want: true},
		{name: "profile remove", args: []string{"profile", "remove", "profile"}, want: true},
		{name: "profile list", args: []string{"profile", "list"}, want: false},
		{name: "profile show", args: []string{"profile", "show", "profile"}, want: false},
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
		{name: "bare profile", args: []string{"profile"}, want: false},
		{name: "bare route", args: []string{"route"}, want: false},
		{name: "bare adapter", args: []string{"adapter"}, want: false},
		{name: "adapter enable", args: []string{"adapter", "enable", "codex"}, want: true},
		{name: "adapter disable", args: []string{"adapter", "disable", "codex"}, want: true},
		{name: "bare config", args: []string{"config"}, want: false},
		{name: "config import", args: []string{"config", "import", "path"}, want: true},
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
		t.Fatal("an interactive terminal with no profiles should trigger the onboarding wizard lock")
	}

	nonInteractive := App{Config: configuration.NewStore(filepath.Join(t.TempDir(), "missing.toml")), Interactive: false}
	if requiresConfigurationLock(&nonInteractive, NewRoot(&nonInteractive)) {
		t.Fatal("a non-interactive session with no profiles must not take a mutation lock")
	}

	path := filepath.Join(t.TempDir(), "configuration.toml")
	store := configuration.NewStore(path)
	cfg := configuration.NewConfig()
	cfg.Accounts["dmx"] = configuration.Account{Label: "DMX", Endpoints: configuration.Endpoints{OpenAIResponses: "https://example.test/v1"}}
	cfg.Profiles["gpt"] = configuration.Profile{Label: "GPT", Account: "dmx", Client: configuration.ClientCodex, Model: "gpt-test"}
	cfg.Routes[configuration.ClientCodex] = "gpt"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	populated := App{Config: store, Interactive: true}
	if requiresConfigurationLock(&populated, NewRoot(&populated)) {
		t.Fatal("an already-configured store must not trigger the onboarding wizard lock")
	}
}
