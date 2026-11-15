package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/adapter/claude"
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/adapter/hermes"
	"aigw-cli/internal/cli"
	"aigw-cli/internal/clients"
	"aigw-cli/internal/domain"
)

// claudeOnOneAndCodexOnTwo saves a configuration in which Claude Code, on
// provider one, and Codex, on provider two, are both enabled, over a keychain
// whose worker is the program's own and holds provider two's Token. slots
// name the items that another program stored. It returns Codex's file.
func claudeOnOneAndCodexOnTwo(t *testing.T, slots ...string) (app *cli.App, out *bytes.Buffer, items *keychain, target string) {
	t.Helper()
	app, out, _, _, _ = testApp(t, "")
	items = withItemsOfAnotherProgram(t, app, slots...)
	items.bindReader(app)
	items.hold(t, "two", fixtureToken)
	target = guidedCodexTarget(t, app)
	cfg := domain.NewConfig(clients.Admission())
	cfg.SetSelectedModel(claude.ID, addProviderModel(&cfg, "one", "One", domain.Endpoints{Anthropic: "https://one.test"}, claude.ID, "claude-test"), "")
	cfg.SetClientActivation(claude.ID, true, executableFixture(t, "claude"), nil)
	cfg.SetSelectedModel(codex.ID, addProviderModel(&cfg, "two", "Two", domain.Endpoints{Responses: "https://two.test/v1"}, codex.ID, "gpt-test"), "")
	cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
	if err := app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return app, out, items, target
}

// waitingDocument decodes what the documents of the commands that reconcile
// several clients share: the next step, and in a preview the client files.
type waitingDocument struct {
	OK          bool   `json:"ok"`
	Source      string `json:"source"`
	NextAction  string `json:"next_action"`
	NextDetail  string `json:"next_detail"`
	ClientFiles []struct {
		Client string `json:"client"`
	} `json:"client_files"`
}

// requireNextStep requires that a command that succeeded names fix as its one
// next step: in text under Next, and in a document as guidance, since no
// script can run it as it stands. It returns the document.
func requireNextStep(t *testing.T, args []string, out *bytes.Buffer, fix string) (document waitingDocument) {
	t.Helper()
	if !slices.Contains(args, "--json") {
		if text := strings.Join(strings.Fields(out.String()), " "); !strings.Contains(text, "Next "+fix) {
			t.Fatalf("%v does not name the fix as its next step:\n%s", args, out)
		}
		return document
	}
	if err := json.Unmarshal(out.Bytes(), &document); err != nil || !document.OK || document.NextAction != "" || document.NextDetail != fix {
		t.Fatalf("%v wrote %s: %v", args, out, err)
	}
	return document
}

// requireLocalCheck runs the local check of client, or of every client, and
// requires that it passes. With synchronize, the client's file is not written
// yet: the check stops, names `aigw sync` as its fix, and passes once that
// ran, since storing a Token writes no client file.
func requireLocalCheck(t *testing.T, app *cli.App, out *bytes.Buffer, synchronize bool, client ...string) {
	t.Helper()
	check := append(append([]string{"check"}, client...), "--depth", "local")
	out.Reset()
	err := cli.Execute(app, check)
	if synchronize {
		if text := strings.Join(strings.Fields(out.String()), " "); err == nil || !strings.Contains(text, "Next aigw sync") {
			t.Fatalf("%v before the client's file is written = %v\n%s", check, err, out)
		}
		out.Reset()
		if err := cli.Execute(app, []string{"sync"}); err != nil {
			t.Fatalf("sync = %v\n%s", err, out)
		}
		out.Reset()
		err = cli.Execute(app, check)
	}
	if err != nil {
		t.Fatalf("%v = %v\n%s", check, err, out)
	}
}

// plannedFiles returns the rows under Client files in the text of a preview,
// up to the credential reader, which the preview names next.
func plannedFiles(out *bytes.Buffer) string {
	_, rows, _ := strings.Cut(out.String(), "Client files")
	rows, _, _ = strings.Cut(rows, "Credential reader")
	return rows
}

// requireClaudeFileAlonePlanned requires that a preview, in text or as a
// document, plans a file of Claude Code and none of Codex.
func requireClaudeFileAlonePlanned(t *testing.T, args []string, out *bytes.Buffer, document bool) {
	t.Helper()
	planned := []string{plannedFiles(out)}
	if document {
		var report waitingDocument
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatalf("%v wrote %s: %v", args, out, err)
		}
		planned = planned[:0]
		for _, file := range report.ClientFiles {
			planned = append(planned, file.Client)
		}
	}
	if files := strings.ToLower(strings.Join(planned, " ")); !strings.Contains(files, claude.ID) || strings.Contains(files, codex.ID) {
		t.Fatalf("%v plans the client files %q; want Claude Code's and not Codex's:\n%s", args, planned, out)
	}
}

// `aigw sync` reconciles every enabled client. A client whose provider's
// Keychain item another program stored waits, as it does for a missing Token:
// the command succeeds, in text, as JSON and as a preview, names the item's
// fix as its next step, leaves that client's file alone and reads no Token of
// that provider, while it writes the file of a client whose provider's Token
// the keychain holds. Followed as printed, the fix stores the Token, and the
// next sync writes the first client's file too.
func TestSyncLeavesTheClientOfAnItemOfAnotherProgramWaiting(t *testing.T) {
	fix := refusalFix("one", "`aigw provider rotate one`")
	for _, flags := range [][]string{nil, {"--json"}, {"--dry-run"}, {"--dry-run", "--json"}} {
		t.Run(strings.Join(append([]string{"sync"}, flags...), " "), func(t *testing.T) {
			app, out, items, target := claudeOnOneAndCodexOnTwo(t, "one")
			native := readFile(t, target)
			args := append([]string{"sync"}, flags...)
			if err := cli.Execute(app, args); err != nil {
				t.Fatalf("%v = %v\n%s", args, err, out)
			}
			document := requireNextStep(t, args, out, fix)
			preview := slices.Contains(flags, "--dry-run")
			want := []string{"empty one", "empty two", "exists one", "exists two"}
			if !preview {
				want = append(want, "reader read two")
			}
			operations := items.operations(t)
			slices.Sort(operations)
			if !slices.Equal(operations, want) {
				t.Fatalf("%v ran %q on the keychain; want %q", args, operations, want)
			}
			if claudeFile(t, app) != nil || bytes.Equal(readFile(t, target), native) != preview {
				t.Fatalf("%v wrote the file of the client that waits, or did not treat Codex's file as a preview (%t) does:\n%s", args, preview, readFile(t, target))
			}
			if preview {
				for _, file := range document.ClientFiles {
					if file.Client != codex.ID {
						t.Fatalf("%v plans a file of %s, which waits", args, file.Client)
					}
				}
				if strings.Contains(out.String(), "settings.json") || (slices.Contains(flags, "--json") && len(document.ClientFiles) == 0) {
					t.Fatalf("%v plans Claude Code's file, or no file of Codex:\n%s", args, out)
				}
				return
			}
			if slices.Contains(flags, "--json") {
				return
			}
			storeTokenAsPrinted(t, app, out, items)
			out.Reset()
			if err := cli.Execute(app, args); err != nil || !bytes.Contains(claudeFile(t, app), []byte("https://one.test")) ||
				!slices.Contains(items.operations(t), "reader read one") {
				t.Fatalf("%v after the fix = %v; the keychain ran %q; Claude Code's file:\n%s", args, err, items.operations(t), claudeFile(t, app))
			}
		})
	}
}

// A Token slot that is there and holds nothing is a missing Token to `aigw
// sync`: the client waits with the step that stores one, no reader reads the
// empty slot, and the sync completes for the client whose Token is stored.
func TestSyncCountsAnEmptyTokenAsMissing(t *testing.T) {
	app, out, items, target := claudeOnOneAndCodexOnTwo(t)
	items.hold(t, "one", "")
	native := readFile(t, target)
	args := []string{"sync"}
	if err := cli.Execute(app, args); err != nil {
		t.Fatalf("%v = %v\n%s", args, err, out)
	}
	if text := strings.Join(strings.Fields(out.String()), " "); !strings.Contains(text, "Next run `aigw provider rotate one`") {
		t.Fatalf("%v does not name the step that stores the Token:\n%s", args, out)
	}
	operations := items.operations(t)
	slices.Sort(operations)
	if want := []string{"empty one", "empty two", "exists one", "exists two", "reader read two"}; !slices.Equal(operations, want) {
		t.Fatalf("%v ran %q on the keychain; want %q", args, operations, want)
	}
	if claudeFile(t, app) != nil || bytes.Equal(readFile(t, target), native) {
		t.Fatalf("%v wrote the file of the client that waits, or not Codex's:\n%s", args, readFile(t, target))
	}
}

// `aigw config rollback` restores a configuration and reconciles the clients
// that it changes. A client whose provider's Keychain item another program
// stored waits: the rollback succeeds, in text and as JSON, restores the
// configuration, leaves that client's file as it was, installs no reader and
// reads no Token, and names the item's fix as its next step in place of the
// local check, which would stop at the same item. The restored configuration
// holds the provider, so the fix ends with `aigw provider rotate`. Followed
// as printed, it stores the Token, and the local check then passes: at once
// where the client's file already is the restored selection's, and after the
// `aigw sync` that the check names where the rollback had that file to write.
func TestConfigRollbackLeavesTheClientOfAnItemOfAnotherProgramWaiting(t *testing.T) {
	fix := refusalFix("one", "`aigw provider rotate one`")
	for _, test := range []struct {
		name string
		// placed says that Claude Code's file is in place for the selection
		// that the rollback restores.
		placed bool
		// change is the last change, whose backup the rollback restores.
		change func(cfg *domain.Config)
	}{
		{name: "the current configuration holds the provider and the file is the restored selection's", placed: true,
			change: func(cfg *domain.Config) {
				cfg.SetSelectedModel(claude.ID, domain.NewModelRef("one", "claude-next"), "")
			}},
		{name: "only the restored configuration holds the provider and the client has no file",
			change: func(cfg *domain.Config) { *cfg = domain.NewConfig(clients.Admission()) }},
	} {
		for _, flags := range [][]string{nil, {"--json"}} {
			t.Run(strings.Join(append([]string{test.name}, flags...), " "), func(t *testing.T) {
				app, out, items, installed := claudeOnOne(t, test.placed, "one")
				cfg, err := app.Config.Load()
				if err != nil {
					t.Fatal(err)
				}
				cfg.SetClientActivation(claude.ID, true, installed, nil)
				first := cfg.SelectedModel(claude.ID)
				if err := app.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				test.change(&cfg)
				if err := app.Config.Save(cfg); err != nil {
					t.Fatal(err)
				}
				file := claudeFile(t, app)

				args := append([]string{"config", "rollback", "--last-change"}, flags...)
				if err := cli.Execute(app, args); err != nil {
					t.Fatalf("%v = %v\n%s", args, err, out)
				}
				if document := requireNextStep(t, args, out, fix); slices.Contains(flags, "--json") && document.Source != "previous_backup" {
					t.Fatalf("%v restored from %q", args, document.Source)
				}
				requireObservedOnly(t, items, args)
				restored, err := app.Config.Load()
				if err != nil || restored.SelectedModel(claude.ID) != first || !restored.Clients[claude.ID].Enabled {
					t.Fatalf("%v restored the selection %v, %v; want %v, enabled", args, restored.SelectedModel(claude.ID), err, first)
				}
				reader, err := app.ResolveCredentialPath()
				if _, statErr := os.Stat(reader); err != nil || !os.IsNotExist(statErr) || !bytes.Equal(claudeFile(t, app), file) {
					t.Fatalf("%v installed a reader at %s (%v, %v) or changed the file of the client that waits", args, reader, err, statErr)
				}
				if slices.Contains(flags, "--json") {
					return
				}
				storeTokenAsPrinted(t, app, out, items)
				requireLocalCheck(t, app, out, !test.placed)
			})
		}
	}
}

// A preview of a rollback plans the clients that the rollback writes: none
// for a client whose provider's Keychain item another program stored. It
// reads no Token and changes nothing.
func TestAConfigRollbackPreviewPlansNoFileOfTheClientThatWaits(t *testing.T) {
	for _, flags := range [][]string{nil, {"--json"}} {
		t.Run(strings.Join(append([]string{"preview"}, flags...), " "), func(t *testing.T) {
			app, out, items, installed := claudeOnOne(t, false, "one")
			cfg, err := app.Config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.SetClientActivation(claude.ID, true, installed, nil)
			for _, saved := range []domain.Config{cfg, domain.NewConfig(clients.Admission())} {
				if err := app.Config.Save(saved); err != nil {
					t.Fatal(err)
				}
			}
			configuration := readFile(t, app.Config.Path())
			args := append([]string{"config", "rollback", "--last-change", "--dry-run"}, flags...)
			if err := cli.Execute(app, args); err != nil {
				t.Fatalf("%v = %v\n%s", args, err, out)
			}
			requireObservedOnly(t, items, args)
			if strings.Contains(out.String(), "settings.json") || strings.Contains(out.String(), `"client_files"`) ||
				strings.Contains(out.String(), "Credential reader") || strings.Contains(out.String(), `"credential_entrypoint"`) {
				t.Fatalf("%v plans a file or a reader for the client that waits:\n%s", args, out)
			}
			if claudeFile(t, app) != nil || !bytes.Equal(readFile(t, app.Config.Path()), configuration) {
				t.Fatalf("%v wrote Claude Code's file or changed the configuration", args)
			}
		})
	}
}

// A team setup reconciles the clients whose files its change rewrites, here
// Codex, whose model on provider one the team file relabels. Provider one's
// Keychain item another program stored, so Codex waits: the setup succeeds,
// in text, as JSON and as previews, stores the Token of the provider that it
// names, shows the item's title and fix in the rows of provider one's Token
// and of Codex, leaves Codex's file as it was and reads no Token of provider
// one. A preview plans no file of Codex. Followed as printed, the fix stores
// the Token; Codex's local check then names `aigw sync`, which writes the
// file that the setup left, and passes.
func TestASetupLeavesTheClientWhoseFileItRewritesWaiting(t *testing.T) {
	for _, flags := range [][]string{nil, {"--json"}, {"--dry-run"}, {"--dry-run", "--json"}} {
		t.Run(strings.Join(append([]string{"setup"}, flags...), " "), func(t *testing.T) {
			app, out, _, _, _ := testApp(t, fixtureToken+"\n")
			items := withItemsOfAnotherProgram(t, app, "one")
			items.bindReader(app)
			target := guidedCodexTarget(t, app)
			cfg := domain.NewConfig(clients.Admission())
			cfg.SetSelectedModel(codex.ID, addProviderModel(&cfg, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, codex.ID, "gpt-test"), "")
			cfg.SetClientActivation(codex.ID, true, executableFixture(t, "codex"), []string{target})
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			configuration, native := readFile(t, app.Config.Path()), readFile(t, target)
			preview, document := slices.Contains(flags, "--dry-run"), slices.Contains(flags, "--json")
			args := append([]string{"setup", "--from", relabelingTeamFile(t), "--provider", "gateway", "--token-stdin"}, flags...)
			if err := cli.Execute(app, args); err != nil {
				t.Fatalf("%v = %v\n%s", args, err, out)
			}
			requireWaitingRows(t, args, out, document, codex.ID, "Codex one/gpt-test")
			for _, operation := range items.operations(t) {
				if strings.HasSuffix(operation, " one") && operation != "exists one" && operation != "empty one" {
					t.Fatalf("%v ran %q on the item of provider one", args, operation)
				}
			}
			if !bytes.Equal(readFile(t, target), native) {
				t.Fatalf("%v changed the file of the client that waits:\n%s", args, readFile(t, target))
			}
			stored, storeErr := items.Get("gateway")
			if preview {
				requireClaudeFileAlonePlanned(t, args, out, document)
				if storeErr == nil || !bytes.Equal(readFile(t, app.Config.Path()), configuration) {
					t.Fatalf("%v stored a Token for gateway (%v) or changed the configuration:\n%s", args, storeErr, out)
				}
				return
			}
			after, err := app.Config.Load()
			if err != nil || storeErr != nil || stored != fixtureToken || after.Models[domain.NewModelRef("one", "gpt-test")].Label != "Relabeled" {
				t.Fatalf("%v stored gateway's Token %t, %v; the relabeled model is %+v, %v", args, stored == fixtureToken, storeErr, after.Models[domain.NewModelRef("one", "gpt-test")], err)
			}
			if document {
				return
			}
			storeTokenAsPrinted(t, app, out, items)
			requireLocalCheck(t, app, out, true, codex.ID)
			if !bytes.Contains(readFile(t, target), []byte("Relabeled")) && !bytes.Contains(readFile(t, target), []byte("https://one.test/v1")) {
				t.Fatalf("after the fix and the sync, Codex's file is not written:\n%s", readFile(t, target))
			}
		})
	}
}

// Hermes lists, beside the provider that it selects, every other provider
// that it can reach. `aigw sync` leaves out one whose Token slot holds an
// item of another program or nothing, as it leaves out one without a Token:
// the credential command that Hermes would run for it would return no Token.
// For that it asks the worker once whether each such slot is there and
// whether it is empty, and the one Token that is read is the selected
// provider's, by the reader.
func TestSyncListsNoHermesProviderWhoseTokenAIGWCannotUse(t *testing.T) {
	for _, state := range []string{"an item of another program", "an empty Token", "a stored Token"} {
		t.Run(state, func(t *testing.T) {
			app, out, _, _, _ := testApp(t, "")
			var slots []string
			if state == "an item of another program" {
				slots = []string{"two"}
			}
			items := withItemsOfAnotherProgram(t, app, slots...)
			items.bindReader(app)
			items.hold(t, "one", fixtureToken)
			switch state {
			case "an empty Token":
				items.hold(t, "two", "")
			case "a stored Token":
				items.hold(t, "two", fixtureToken)
			}
			target := filepath.Join(t.TempDir(), "hermes", "config.yaml")
			installed := executableFixture(t, "hermes")
			app.Discovery = fakeDiscovery{result: discovery.Result{
				Executables: map[string]string{hermes.ID: installed},
				Surfaces:    []discovery.Surface{{ID: "hermes-home-default", ConfigPath: target}},
			}}
			cfg := domain.NewConfig(clients.Admission())
			cfg.SetSelectedModel(hermes.ID, addProviderModel(&cfg, "one", "One", domain.Endpoints{Responses: "https://one.test/v1"}, hermes.ID, "gpt-one"), "")
			addProviderModel(&cfg, "two", "Two", domain.Endpoints{Responses: "https://two.test/v1"}, hermes.ID, "gpt-two")
			cfg.SetClientActivation(hermes.ID, true, installed, []string{target})
			if err := app.Config.Save(cfg); err != nil {
				t.Fatal(err)
			}
			args := []string{"sync"}
			if err := cli.Execute(app, args); err != nil {
				t.Fatalf("%v = %v\n%s", args, err, out)
			}
			file := readFile(t, target)
			if listed := bytes.Contains(file, []byte("aigw-two-responses")); !bytes.Contains(file, []byte("aigw-one-responses")) || listed != (state == "a stored Token") {
				t.Fatalf("%v lists provider two for Hermes %t:\n%s", args, listed, file)
			}
			operations := items.operations(t)
			slices.Sort(operations)
			if want := []string{"empty one", "empty two", "exists one", "exists two", "reader read one"}; !slices.Equal(operations, want) {
				t.Fatalf("%v ran %q on the keychain; want %q", args, operations, want)
			}
		})
	}
}
