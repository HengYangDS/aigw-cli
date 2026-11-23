package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// This file holds the tests of one client file that two configurations name
// in two ways: through a symbolic link, and by the path with every link
// resolved. Whether a command keeps or drops a client's file is decided by
// the file's identity, never by how its path is spelled. These are the
// twelfth review's findings 1 (a rollback without the Token stripped AIGW's
// settings and record from the file that the client still uses) and 4 (an
// edit that names the file in another spelling succeeded or was refused by
// whether the Token was stored), for Codex and for Hermes, each run once with
// the Token and once without it.

// fileSpelling names a client's file through a symbolic link that the test
// makes in a temporary directory of its own.
type fileSpelling struct {
	name string
	link func(t *testing.T, file string) string
}

var (
	// throughAFileLink names the file by a link to the file itself.
	throughAFileLink = fileSpelling{"a link to the file", func(t *testing.T, file string) string {
		return symlinkTo(t, file)
	}}
	// throughAFolderLink names the file inside a link to its folder.
	throughAFolderLink = fileSpelling{"a link to its folder", func(t *testing.T, file string) string {
		return filepath.Join(symlinkTo(t, filepath.Dir(file)), filepath.Base(file))
	}}
)

// symlinkTo makes a symbolic link to target and returns the link's path.
// Windows lets only a privileged or developer-mode account create one, so
// there a test that cannot is skipped; everywhere else the failure fails it.
func symlinkTo(t *testing.T, target string) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), filepath.Base(target))
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("this account cannot create a symbolic link: %v", err)
		}
		t.Fatal(err)
	}
	return link
}

// respelling is the change of the path by which a client's configuration
// names its file: before the command under test the configuration names it
// one way, and the command names it the other.
type respelling struct {
	name   string
	toLink bool
}

var respellings = []respelling{
	{"from the resolved path to the link", true},
	{"from the link to the resolved path", false},
}

// paths returns the path that the configuration names the file by before the
// command and the one after it.
func (change respelling) paths(t *testing.T, spelling fileSpelling, file string) (from, to string) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	link := spelling.link(t, file)
	if change.toLink {
		return resolved, link
	}
	return link, resolved
}

// identityClient is a client of these tests: its world, a choice of another
// model in its first file, and the links that name that file. Both clients
// write the file that a link names, so a link to the file and a link to its
// folder each name the client's file before and after a write. Hermes has
// one home, so the cases of a second file apply to Codex alone.
type identityClient struct {
	id        string
	world     func(t *testing.T) selectionWorld
	choose    func(t *testing.T, w selectionWorld) selectionWorld
	spellings []fileSpelling
}

func identityClients() map[string]identityClient {
	pickInCodex := func(t *testing.T, w selectionWorld) selectionWorld { return w.picking(t, 0, basicString, "gpt-terra") }
	return map[string]identityClient{
		"codex": {
			id: codex.ID, world: func(t *testing.T) selectionWorld { return codexWorld(t, 1) }, choose: pickInCodex,
			spellings: []fileSpelling{throughAFileLink, throughAFolderLink},
		},
		// The first file is named another way, and the second as it is.
		"codex with a second file": {
			id: codex.ID, world: func(t *testing.T) selectionWorld { return codexWorld(t, 2) }, choose: pickInCodex,
			spellings: []fileSpelling{throughAFileLink},
		},
		"hermes": {
			id: hermes.ID, world: hermesPlain,
			choose: func(t *testing.T, w selectionWorld) selectionWorld {
				return w.choosing(t, "dmxapi", "anthropic", "claude-opus-5-5", plainScalars)
			},
			spellings: []fileSpelling{throughAFileLink, throughAFolderLink},
		},
	}
}

// eachRespelling runs test for every client, spelling, respelling and whether
// a model was chosen in the file, each in a parallel subtest of its own.
func eachRespelling(t *testing.T, test func(t *testing.T, client identityClient, spelling fileSpelling, change respelling, chosen bool)) {
	t.Helper()
	clients := identityClients()
	for _, name := range slices.Sorted(mapKeys(clients)) {
		client := clients[name]
		for _, spelling := range client.spellings {
			for _, change := range respellings {
				for _, chosen := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/a choice=%t", name, spelling.name, change.name, chosen), func(t *testing.T) {
						t.Parallel()
						test(t, client, spelling, change, chosen)
					})
				}
			}
		}
	}
}

// naming saves the world's client on targets, with the Token, as
// `aigw client edit --target` does. The world's first file, which targets
// name first, must still be the client's file afterwards: it holds AIGW's
// settings, and its record names what it selects.
func (w selectionWorld) naming(t *testing.T, targets ...string) {
	t.Helper()
	args := []string{"client", "edit", w.client}
	for _, target := range targets {
		args = append(args, "--target", target)
	}
	w.run(t, args...)
	if got := selection(t, w.app, w.client).Targets; !slices.Equal(got, targets) {
		t.Fatalf("the client is on %q, not on %q", got, targets)
	}
	if after := w.snapshot(t); after.files[0] == nil || !w.keepsAIGWsPart(after.files[0]) || !after.follows[0] {
		t.Fatalf("with the Token, the edit that names the client's file as %s left it without AIGW's settings or without a record that names what it selects: %s",
			targets[0], shownLine(after))
	}
}

// identityTwins runs a command in two worlds that prepare builds with every
// Token: the first keeps them, and the second runs without the Token of the
// stored selection's provider. Each run is checked by the one sentence
// (holds); neither may be refused, and every file of the world, which the
// command keeps, must still hold AIGW's settings. The twins are compared by
// what a Token must not decide (twinned).
func identityTwins(t *testing.T, prepare func(t *testing.T) (selectionWorld, selectionCommand)) {
	t.Helper()
	var runs [2]selectionRun
	var command selectionCommand
	account := ""
	for index, token := range []bool{true, false} {
		var world selectionWorld
		world, command = prepare(t)
		world.setToken(t, token)
		run := selectionRun{before: world.snapshot(t)}
		world.output.Reset()
		run.err = cli.Execute(world.app, command.args)
		run.after = world.holds(t, run.before, command, run.err)
		if run.err != nil {
			t.Errorf("token=%t: %s was refused: %v", token, command.name, run.err)
		} else {
			for file, data := range run.after.files {
				if !world.keepsAIGWsPart(data) {
					t.Errorf("token=%t: %s kept file %d of the client and removed AIGW's settings from it:\n%s", token, command.name, file, data)
				}
			}
		}
		if t.Failed() {
			t.Logf("token=%t: before: %s\n  after:  %s\n%s said:\n%s", token, shownLine(run.before), shownLine(run.after), command.name, world.output)
		}
		runs[index], account = run, world.account
	}
	runs[1].twinned(t, runs[0], command, account)
}

// An edit names the file that the client has in another spelling.
//
// Before: the client's configuration names its file one way; the file
// selects the stored model and AIGW's record names it, or the person chose
// another model in the file. The person runs `aigw client edit <client>
// --target <the same file, named the other way>`, naming a second Codex file
// as it is. The edit drops no file: it is saved, with the Token and without
// it, the file keeps AIGW's settings, and its record names what it selects.
// The twelfth review found the edit refused without the Token, because the
// file in its old spelling read as dropped.
func TestFileIdentityAnEditThatNamesTheFileAnotherWay(t *testing.T) {
	eachRespelling(t, func(t *testing.T, client identityClient, spelling fileSpelling, change respelling, chosen bool) {
		identityTwins(t, func(t *testing.T) (selectionWorld, selectionCommand) {
			world := client.world(t)
			from, to := change.paths(t, spelling, world.files[0])
			world.naming(t, append([]string{from}, world.files[1:]...)...)
			if chosen {
				world = client.choose(t, world)
			}
			targets := append([]string{to}, world.files[1:]...)
			args := []string{"client", "edit", client.id}
			for _, target := range targets {
				args = append(args, "--target", target)
			}
			return world, selectionCommand{name: "the edit", args: append(args, "--json"), leaves: func(after selectionSnapshot) string {
				if slices.Equal(after.binding.Targets, targets) {
					return ""
				}
				return fmt.Sprintf("the client is on %q, not on %q", after.binding.Targets, targets)
			}}
		})
	})
}

// An edit names Codex's default home, which does not exist yet, in another
// spelling of the folder that holds it: through a link to that folder, or
// with the folder named in another case on a volume that ignores case.
//
// Before: Codex is enabled on its default home, which AIGW creates when it
// writes it, and the provider's Token is missing, so no synchronization has
// created it. The person stores the Token, or not, and runs `aigw client edit
// codex --target <the same file in the other spelling>`. The file does not
// exist, and it is the same file once it does: its folder is the home's
// folder by the operating system, and its name is the same. So the edit drops
// no file and ends alike with the Token and without it: it is saved, and with
// the Token the home is created as the default home.
func TestFileIdentityAnEditThatNamesTheMissingDefaultHomeInAnotherSpelling(t *testing.T) {
	spellings := map[string]func(t *testing.T, home string) string{
		"through a linked folder": func(t *testing.T, home string) string {
			return filepath.Join(symlinkTo(t, home), ".codex", "config.toml")
		},
		"its folder in another case": func(t *testing.T, home string) string {
			other := filepath.Join(home, ".CODEX")
			if left, err := os.Stat(filepath.Join(home, ".codex")); err != nil {
				t.Fatal(err)
			} else if right, err := os.Stat(other); err != nil || !os.SameFile(left, right) {
				t.Skip("this volume keeps case")
			}
			return filepath.Join(other, "config.toml")
		},
	}
	for _, name := range slices.Sorted(mapKeys(spellings)) {
		t.Run(name, func(t *testing.T) {
			type outcome struct {
				err     error
				stored  string
				created bool
			}
			var outcomes [2]outcome
			for index, token := range []bool{true, false} {
				app, output, store, _, _ := testApp(t, "")
				bindCredentialReader(t, app)
				home := filepath.Join(t.TempDir(), "home")
				config := filepath.Join(home, ".codex", "config.toml")
				if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
					t.Fatal(err)
				}
				app.Discovery = fakeDiscovery{result: discovery.Result{Surfaces: []discovery.Surface{{ID: string(codex.HomeDefault), ConfigPath: config, Present: true, AutoManaged: true}}}}
				cfg := domain.NewConfig(clients.Admission())
				cfg.Providers["gateway"] = domain.Provider{Label: "Gateway", Endpoints: domain.Endpoints{Responses: "https://gateway.test/v1"}}
				cfg.Models[domain.NewModelRef("gateway", "gpt-sol")] = qualifiedModel("gpt-sol", "gateway", "gpt-sol", domain.ProtocolResponses)
				cfg.SetSelectedModel(codex.ID, domain.NewModelRef("gateway", "gpt-sol"), "")
				cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{config})
				if err := app.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				other := spellings[name](t, home)
				if token {
					if err := store.Set("gateway", "fixture-token"); err != nil {
						t.Fatal(err)
					}
				}
				output.Reset()
				err := cli.Execute(app, []string{"client", "edit", codex.ID, "--target", other, "--json"})
				_, statErr := os.Stat(config)
				outcomes[index] = outcome{err: err, stored: selection(t, app, codex.ID).Model.String(), created: statErr == nil}
				if err != nil {
					t.Errorf("token=%t: the edit that names the missing default home %s was refused: %v\n%s", token, name, err, output)
				}
			}
			if with, without := outcomes[0], outcomes[1]; (with.err == nil) != (without.err == nil) || with.stored != without.stored {
				t.Errorf("the Token decides the edit: with it %+v, without it %+v", with, without)
			}
			if !outcomes[0].created {
				t.Error("with the Token, the edit did not create the default home")
			}
		})
	}
}

// Hermes's file is a symbolic link to a file elsewhere, as a person who keeps
// their Hermes configuration in another folder makes it.
//
// Before: config.yaml in the Hermes home links to a file of Hermes's own in
// another folder. The person sets AIGW up for Hermes with the Token, which
// writes the file; then the person disables Hermes in AIGW, with the Token or
// without it, which restores the file. After each command the link is still a
// link, and the file that it names holds what AIGW wrote, with a record that
// names what the file selects, and after the withdrawal neither AIGW's
// settings nor its record.
func TestFileIdentityHermesWritesThroughALinkToItsFile(t *testing.T) {
	for _, token := range []bool{true, false} {
		t.Run(fmt.Sprintf("token at the withdrawal=%t", token), func(t *testing.T) {
			named := filepath.Join(t.TempDir(), "config.yaml")
			writeFile(t, named, []byte("model:\n  default: native-model\n  provider: openrouter\n"), 0o600)
			target := symlinkTo(t, named)
			app, output, _, _ := shippedHermesAppAt(t, true, target)
			world := hermesWorld(t, app, target)
			world.output = output
			linked := func(after string) {
				t.Helper()
				if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("after %s, Hermes's file is no longer a link (%v), and the file that it named holds:\n%s", after, err, readFile(t, named))
				}
			}
			linked("the setup")
			if data := readFile(t, named); !world.keepsAIGWsPart(data) || !world.follows(t, target) {
				t.Fatalf("the setup did not write the file that the link names with a record that follows it:\n%s", data)
			}
			world.setToken(t, token)
			world.run(t, "client", "disable", hermes.ID)
			linked("the withdrawal")
			if data := readFile(t, named); world.keepsAIGWsPart(data) || world.recordOf(t, target) != nil {
				t.Fatalf("the withdrawal left AIGW's settings or its record in the file that the link names:\n%s", data)
			}
		})
	}
}

// A rollback restores the configuration that named the client's file in the
// other spelling.
//
// Before: the configuration named the client's file one way; with the Token
// the person ran `aigw client edit <client> --target <the same file, named
// the other way>`, and then maybe chose another model in the file. The Token
// is lost, or not, and the person runs `aigw config rollback --last-change`,
// which names the file as before. The rollback drops no file: it restores the
// binding, the file keeps AIGW's settings, and its record names what it
// selects, with the Token and without it. The twelfth review found that
// without the Token the rollback restored the file as one that the client
// had dropped: it removed AIGW's settings and record from it, and a model
// chosen there was no longer seen as a choice.
func TestFileIdentityARollbackToTheFileNamedAnotherWay(t *testing.T) {
	eachRespelling(t, func(t *testing.T, client identityClient, spelling fileSpelling, change respelling, chosen bool) {
		identityTwins(t, func(t *testing.T) (selectionWorld, selectionCommand) {
			world := client.world(t)
			from, to := change.paths(t, spelling, world.files[0])
			world.naming(t, append([]string{from}, world.files[1:]...)...)
			restored := selection(t, world.app, client.id)
			world.restoredBinding = &restored
			world.naming(t, append([]string{to}, world.files[1:]...)...)
			if chosen {
				world = client.choose(t, world)
			}
			return world, selectionCommand{name: "the rollback", args: []string{"config", "rollback", "--last-change", "--json"}, explicit: true, restores: true}
		})
	})
}

// A rollback drops one Codex file and keeps the other, which it names in the
// other spelling.
//
// Before: Codex was on file A alone, named one way. With the Token the person
// ran `aigw client edit codex --target <A, named the other way> --target B`.
// The Token is lost, or not, and the person runs `aigw config rollback
// --last-change`, which drops B. Without the Token the rollback withdraws
// AIGW's part from B alone: B keeps neither AIGW's settings nor its record,
// and A keeps both, with its record naming what A selects.
func TestFileIdentityARollbackThatDropsOneCodexFileKeepsTheOther(t *testing.T) {
	for _, change := range respellings {
		t.Run(change.name, func(t *testing.T) {
			t.Parallel()
			// The one sentence reads the files that Codex keeps; each twin's
			// file B is read here, after both ran.
			var twins []selectionWorld
			var dropped []string
			identityTwins(t, func(t *testing.T) (selectionWorld, selectionCommand) {
				world := codexWorld(t, 2)
				fileA, fileB := world.files[0], world.files[1]
				from, to := change.paths(t, throughAFileLink, fileA)
				world.naming(t, from)
				restored := selection(t, world.app, codex.ID)
				world.restoredBinding = &restored
				world.naming(t, to, fileB)
				world.files = world.files[:1]
				twins, dropped = append(twins, world), append(dropped, fileB)
				return world, selectionCommand{name: "the rollback", args: []string{"config", "rollback", "--last-change", "--json"}, explicit: true, restores: true}
			})
			for index, world := range twins {
				if data := readFile(t, dropped[index]); world.keepsAIGWsPart(data) {
					t.Errorf("token=%t: the rollback dropped file B and left AIGW's settings in it:\n%s", index == 0, data)
				}
				if world.recordOf(t, dropped[index]) != nil {
					t.Errorf("token=%t: the rollback dropped file B and left AIGW's record of it", index == 0)
				}
			}
		})
	}
}
