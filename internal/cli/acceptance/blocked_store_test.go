package cli_test

import (
	"aigw-cli/internal/domain"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/secret"
	"aigw-cli/internal/store"
)

// blockedStoreClient is a Claude Code client whose configuration and Token
// lie in AIGW's folder in the command's own home, as they do in production:
// the configuration file, and the file Token store's secrets folder.
type blockedStoreClient struct {
	app    *cli.App
	output interface {
		Reset()
		String() string
		Bytes() []byte
	}
	folder  string
	secrets string
}

func newBlockedStoreClient(t *testing.T) blockedStoreClient {
	t.Helper()
	app, output, _, _, _ := testApp(t, "")
	bindCredentialReader(t, app)
	folder := filepath.Join(app.Host.Home, "aigw")
	secrets := filepath.Join(folder, "secrets")
	app.Config = store.NewStore(filepath.Join(folder, "configuration.toml"), clients.Admission())
	tokens, err := secret.Select(secret.Selection{Backend: "file", GOOS: runtime.GOOS, Root: secrets, Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	app.Secrets = tokens
	cfg := anthropicConfig()
	cfg.SetSelectedModel(claude.ID, domain.NewModelRef("gateway", "claude-test"), "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	saveAndSync(t, app, output, tokens, cfg)
	return blockedStoreClient{app: app, output: output, folder: folder, secrets: secrets}
}

// next runs command and returns its document's next step for the client, or
// the document's own.
func (c blockedStoreClient) next(t *testing.T, command []string) string {
	t.Helper()
	c.output.Reset()
	_ = cli.Execute(c.app, command)
	var document struct {
		NextAction string       `json:"next_action"`
		NextDetail string       `json:"next_detail"`
		Clients    clientStates `json:"clients"`
	}
	if err := json.Unmarshal(c.output.Bytes(), &document); err != nil {
		t.Fatalf("%v: %v\n%s", command, err, c.output)
	}
	if strings.Contains(c.output.String(), c.app.Host.Home) {
		t.Fatalf("%v names the private home: %s", command, c.output)
	}
	if client, found := document.Clients[claude.ID]; found && client.NextDetail != "" {
		return client.NextDetail
	}
	return document.NextAction + document.NextDetail
}

// copyFolder copies folder, as an operator's copy of it holds it.
func copyFolder(t *testing.T, folder string) string {
	t.Helper()
	copied := filepath.Join(t.TempDir(), filepath.Base(folder))
	err := filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(folder, path)
		info, _ := entry.Info()
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(copied, relative), info.Mode().Perm())
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(filepath.Join(copied, relative), data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	return copied
}

// blockings are the ways that a regular file or a link that leads nowhere
// takes the place of one of AIGW's folders, with the name of what the fix
// moves or removes and how the fix says so.
var blockings = []struct {
	name  string
	block func(t *testing.T, folder string)
	lead  string
	clear func(t *testing.T, folder string)
}{
	{"a file", func(t *testing.T, folder string) { writeFile(t, folder, []byte("x"), 0o600) }, "Move the file",
		func(t *testing.T, folder string) { renameAway(t, folder) }},
	{"a link that leads nowhere", func(t *testing.T, folder string) {
		if err := os.Symlink(folder+".nowhere", folder); err != nil {
			t.Skipf("symbolic links are unavailable: %v", err)
		}
	}, "Remove the link", func(t *testing.T, folder string) { removeAll(t, folder) }},
}

func renameAway(t *testing.T, path string) {
	t.Helper()
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
}

// A file or a link that leads nowhere in the place of AIGW's Token folder is
// named, with the one fix of its condition, on every surface that observes the
// store, never folded into a backend that is unavailable or a Token that is
// missing. Followed literally, the folder put back from its copy, the fix lets
// each command run, and a check pass after the sync that it names.
func TestABlockedTokenFolderNamesItsFixOnEverySurface(t *testing.T) {
	for _, blocking := range blockings {
		for _, surface := range []struct {
			command []string
			check   bool
		}{
			{[]string{"check", claude.ID, "--depth", "local", "--json"}, true},
			{[]string{"status", "--json"}, true},
			{[]string{"sync", "--json"}, false},
			{[]string{"use", claude.ID, "gateway/claude-test", "--json"}, false},
		} {
			t.Run(blocking.name+"/"+surface.command[0], func(t *testing.T) {
				client := newBlockedStoreClient(t)
				copied := copyFolder(t, client.secrets)
				removeAll(t, client.secrets)
				blocking.block(t, client.secrets)
				step := "run the same command again"
				if surface.check {
					step = "run `aigw sync`"
				}
				want := blocking.lead + " ~/aigw/secrets. If it replaced AIGW's folder, put that folder back from a copy; otherwise store each provider's Token again with `aigw provider rotate <provider>`. Then " + step + "."
				if got := client.next(t, surface.command); got != want {
					t.Fatalf("%v names %q\nwant         %q", surface.command, got, want)
				}
				// status's own next step is the same fix, not only the local check.
				if surface.command[0] == "status" {
					var document struct {
						NextAction string `json:"next_action"`
						NextDetail string `json:"next_detail"`
					}
					if err := json.Unmarshal(client.output.Bytes(), &document); err != nil || document.NextAction+document.NextDetail != want {
						t.Fatalf("status's next step = %+v, %v", document, err)
					}
				}
				blocking.clear(t, client.secrets)
				if err := os.Rename(copied, client.secrets); err != nil {
					t.Fatal(err)
				}
				if surface.check {
					client.output.Reset()
					if err := cli.Execute(client.app, []string{"sync"}); err != nil {
						t.Fatalf("the sync that the check named: %v\n%s", err, client.output)
					}
				}
				client.output.Reset()
				if err := cli.Execute(client.app, surface.command); err != nil {
					t.Fatalf("%v after its fix: %v\n%s", surface.command, err, client.output)
				}
			})
		}
	}
}

// A link that leads nowhere in the place of AIGW's configuration folder, or of
// the configuration file itself, is no first run: every command names the
// link to remove, and setup never writes through the link or replaces it with
// a new file. Followed literally, the folder put back from its copy, the
// command runs.
func TestALinkThatLeadsNowhereIsNoFirstRun(t *testing.T) {
	for _, surface := range [][]string{
		{"status", "--json"},
		{"check", claude.ID, "--depth", "local", "--json"},
		{"sync", "--json"},
		{"use", claude.ID, "gateway/claude-test", "--json"},
	} {
		t.Run("the folder/"+surface[0], func(t *testing.T) {
			client := newBlockedStoreClient(t)
			copied := copyFolder(t, client.folder)
			removeAll(t, client.folder)
			blockings[1].block(t, client.folder)
			want := "Remove the link ~/aigw. If it replaced AIGW's folder, put that folder back from a copy; otherwise run `aigw setup`. Then run the same command again."
			if got := client.next(t, surface); got != want {
				t.Fatalf("%v names %q\nwant         %q", surface, got, want)
			}
			removeAll(t, client.folder)
			if err := os.Rename(copied, client.folder); err != nil {
				t.Fatal(err)
			}
			client.output.Reset()
			if err := cli.Execute(client.app, surface); err != nil {
				t.Fatalf("%v after its fix: %v\n%s", surface, err, client.output)
			}
		})
	}
	// The configuration file itself may be a link on purpose, as dotfiles keep
	// it, whose target is only away for now: the fix names that target inside
	// home, and setup never replaces the link. Followed literally, the target
	// made available again, every command runs.
	t.Run("the configuration file", func(t *testing.T) {
		client := newBlockedStoreClient(t)
		config := client.app.Config.Path()
		target := filepath.Join(client.app.Host.Home, "dotfiles", "configuration.toml")
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(config, target+".away"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, config); err != nil {
			t.Skipf("symbolic links are unavailable: %v", err)
		}
		team := filepath.Join(t.TempDir(), "team.toml")
		writeFile(t, team, []byte("version = 8\n"), 0o600)
		want := "Make ~/dotfiles/configuration.toml, which the link ~/aigw/configuration.toml leads to, available again; otherwise remove the link and run `aigw setup`. Then run the same command again."
		surfaces := [][]string{{"setup", "--from", team, "--json"}, {"status", "--json"}, {"check", claude.ID, "--depth", "local", "--json"}, {"sync", "--json"}, {"use", claude.ID, "gateway/claude-test", "--json"}}
		for _, surface := range surfaces {
			if got := client.next(t, surface); got != want {
				t.Fatalf("%v names %q\nwant         %q", surface, got, want)
			}
		}
		if info, err := os.Lstat(config); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("setup replaced the link: %v, %v", info, err)
		}
		if err := os.Rename(target+".away", target); err != nil {
			t.Fatal(err)
		}
		for _, surface := range surfaces[1:] {
			client.output.Reset()
			if err := cli.Execute(client.app, surface); err != nil {
				t.Fatalf("%v after its fix: %v\n%s", surface, err, client.output)
			}
		}
	})
}
