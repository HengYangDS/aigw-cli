package cli_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"aigw-cli/internal/secret"

	"go.yaml.in/yaml/v4"
)

// This file holds the regression tests of a provider's Token that its store
// cannot observe (the Change one-token-usability, from the twelfth review's
// findings 2, 3 and 5). Such a Token counts as an absent one for each client
// that uses it: the client waits, and the command ends as it ends when the
// Token is absent. Each test therefore runs twins that differ only in that
// Token and compares how they end.

// errStoreCannotTell is what a store answers that cannot tell whether a Token
// is there, as a locked keychain answers.
var errStoreCannotTell = errors.New("the keychain is locked")

// storeThatCannotTell stands in front of the world's store. For the accounts
// that it names it cannot tell whether the Token is there, nor read it, as a
// locked keychain cannot; it answers for every other account as the store
// behind it does.
type storeThatCannotTell struct {
	secret.Store
	accounts []string
}

func (s storeThatCannotTell) Exists(account string) (bool, error) {
	if slices.Contains(s.accounts, account) {
		return false, errStoreCannotTell
	}
	return s.Store.Exists(account)
}

func (s storeThatCannotTell) Get(account string) (string, error) {
	if slices.Contains(s.accounts, account) {
		return "", errStoreCannotTell
	}
	return s.Store.Get(account)
}

// unobservableStore is one way in which a store cannot observe a provider's
// Token. hold moves the world's Tokens into a store of that kind: stored
// names the providers whose Token is stored, and unobservable those of them
// whose Token the store cannot observe.
type unobservableStore struct {
	name string
	hold func(t *testing.T, w selectionWorld, stored, unobservable []string)
}

// unobservableStores are AIGW's file credential store, in a temporary folder,
// holding a Token file with the mode 0644, as a copy restored from a backup
// has it, which the store refuses; and a store that cannot tell.
func unobservableStores() []unobservableStore {
	return []unobservableStore{
		{name: "a Token file that the file store refuses", hold: func(t *testing.T, w selectionWorld, stored, unobservable []string) {
			t.Helper()
			root := t.TempDir()
			store, err := secret.Select(secret.Selection{Backend: "file", GOOS: runtime.GOOS, Root: root, Getenv: func(string) string { return "" }})
			if err != nil {
				t.Fatal(err)
			}
			for _, provider := range stored {
				if err := store.Set(provider, "fixture-token"); err != nil {
					t.Fatal(err)
				}
			}
			for _, provider := range unobservable {
				// The selected file backend keeps its Token files in the folder tokens.
				if err := os.Chmod(filepath.Join(root, "tokens", provider), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := store.Exists(provider); err == nil {
					t.Fatalf("the file store still tells whether %s's Token is there", provider)
				}
			}
			w.app.Secrets = store
		}},
		{name: "a store that cannot tell", hold: func(t *testing.T, w selectionWorld, stored, unobservable []string) {
			t.Helper()
			store := secret.NewMemoryStore()
			for _, provider := range stored {
				if err := store.Set(provider, "fixture-token"); err != nil {
					t.Fatal(err)
				}
			}
			w.app.Secrets = storeThatCannotTell{Store: store, accounts: unobservable}
		}},
	}
}

// usabilityOutcome is how a command ends, as twins compare it: whether it
// was refused, the stored selection, how many files the client has, whether
// each file's record names what the file selects, and whether each file
// keeps AIGW's settings.
type usabilityOutcome struct {
	refused  bool
	stored   string
	targets  int
	follows  string
	aigwPart string
}

func usabilityOutcomeOf(t *testing.T, w selectionWorld, run ranIn) usabilityOutcome {
	t.Helper()
	parts := make([]string, 0, len(run.after.files))
	for _, file := range run.after.files {
		parts = append(parts, fmt.Sprint(file != nil && w.keepsAIGWsPart(file)))
	}
	return usabilityOutcome{
		refused: run.err != nil, stored: run.after.stored, targets: len(selection(t, w.app, w.client).Targets),
		follows: fmt.Sprint(run.after.follows), aigwPart: strings.Join(parts, ","),
	}
}

// hermesEntries names, sorted, the provider entries of Hermes's file at path.
func hermesEntries(t *testing.T, path string) []string {
	t.Helper()
	var document struct {
		Providers map[string]yaml.Node `yaml:"providers"`
	}
	if err := yaml.Unmarshal(readFile(t, path), &document); err != nil {
		t.Fatalf("Hermes's file %s is not YAML: %v", path, err)
	}
	return slices.Sorted(mapKeys(document.Providers))
}

// The twelfth review's finding 2. Hermes chose dmxapi/claude-opus-5-5 on the
// Anthropic protocol in its file, beside a record and a stored selection of
// dmxapi/gpt-6.2-sol on Responses; dmxapi and ucloud have usable Tokens.
// aihubmix, a provider of the configuration that Hermes does not select, has
// no Token, or one that its store cannot observe. A command about another
// client, one that names nothing, `aigw sync` and an edit of Hermes's own
// setting each end alike in both twins: Hermes is written without
// aihubmix's models, as for a provider without a Token, and AIGW's record of
// Hermes's file names the model that the file selects. The edit is
// `--direct`, since a forwarding service relays no Anthropic traffic.
func TestHermesLeavesOutAnotherProviderWhoseTokenCannotBeObserved(t *testing.T) {
	t.Parallel()
	commands := map[string][]string{
		"another client": {"use", "claude", "ucloud/claude-opus-5-5", "--json"},
		"names nothing":  {"provider", "add", "extra", "--endpoint", "responses=https://extra.test/v1", "--token-stdin", "--json"},
		"sync":           {"sync", "--json"},
		"own setting":    {"client", "edit", "hermes", "--direct", "--json"},
	}
	for _, kind := range unobservableStores() {
		for _, name := range slices.Sorted(mapKeys(commands)) {
			t.Run(kind.name+"/"+name, func(t *testing.T) {
				outcomes, entries := map[bool]usabilityOutcome{}, map[bool][]string{}
				for _, unobservable := range []bool{false, true} {
					world := hermesPlain(t).choosing(t, "dmxapi", "anthropic", "claude-opus-5-5", plainScalars)
					if unobservable {
						kind.hold(t, world, []string{"dmxapi", "ucloud", "aihubmix"}, []string{"aihubmix"})
					} else {
						kind.hold(t, world, []string{"dmxapi", "ucloud"}, nil)
					}
					if name == "names nothing" {
						world.app.In = strings.NewReader("extra-token\n")
					}
					run := runIn(t, world, commands[name]...)
					outcomes[unobservable] = usabilityOutcomeOf(t, world, run)
					entries[unobservable] = hermesEntries(t, world.files[0])
					t.Logf("aihubmix's Token unobservable=%t: err=%v\n  before: %s\n  after:  %s", unobservable, run.err, shownLine(run.before), shownLine(run.after))
					if run.err != nil {
						t.Errorf("unobservable=%t: %v refused: %v\n%s", unobservable, commands[name], run.err, run.output)
						continue
					}
					if !run.after.follows[0] {
						t.Errorf("unobservable=%t: %v left Hermes's record behind its file: %s", unobservable, commands[name], shownLine(run.after))
					}
				}
				if outcomes[false] != outcomes[true] {
					t.Errorf("whether aihubmix's Token can be observed decides the outcome: absent %+v; unobservable %+v", outcomes[false], outcomes[true])
				}
				if !slices.Equal(entries[false], entries[true]) || slices.ContainsFunc(entries[true], func(entry string) bool { return strings.HasPrefix(entry, "aigw-aihubmix-") }) {
					t.Errorf("Hermes's file lists %q beside an unobservable Token of aihubmix, and %q beside none", entries[true], entries[false])
				}
			})
		}
	}
}

// The twelfth review's finding 3. Codex is on one file that shows
// gateway/gpt-sol with its record ("recorded"), or that chose gpt-terra
// beside that record ("chosen"). gateway, the provider of Codex's selection,
// has no Token, or one that its store cannot observe. `aigw sync` and its
// preview end alike in both twins: Codex waits for its Token. The result of
// the unobservable twin names the store's failure, as the state unavailable,
// and a next step for it, not the step that stores a missing Token.
func TestSyncWaitsForASelectedTokenThatCannotBeObserved(t *testing.T) {
	t.Parallel()
	states := map[string]func(t *testing.T) selectionWorld{
		"recorded": func(t *testing.T) selectionWorld { return codexWorld(t, 1) },
		"chosen":   func(t *testing.T) selectionWorld { return codexWorld(t, 1).picking(t, 0, basicString, "gpt-terra") },
	}
	for _, kind := range unobservableStores() {
		for _, state := range slices.Sorted(mapKeys(states)) {
			for _, args := range [][]string{{"sync", "--json"}, {"sync", "--dry-run", "--json"}} {
				t.Run(fmt.Sprintf("%s/%s/%s", kind.name, state, strings.Join(args, " ")), func(t *testing.T) {
					outcomes := map[bool]usabilityOutcome{}
					for _, unobservable := range []bool{false, true} {
						world := states[state](t)
						if unobservable {
							kind.hold(t, world, []string{"gateway", "spare"}, []string{"gateway"})
						} else {
							kind.hold(t, world, []string{"spare"}, nil)
						}
						run := runIn(t, world, args...)
						outcomes[unobservable] = usabilityOutcomeOf(t, world, run)
						t.Logf("gateway's Token unobservable=%t: err=%v\n  before: %s\n  after:  %s\n%s", unobservable, run.err, shownLine(run.before), shownLine(run.after), run.output)
						if run.err != nil {
							t.Errorf("unobservable=%t: %v refused: %v", unobservable, args, run.err)
							continue
						}
						var document struct {
							followUp
							State string `json:"state"`
						}
						run.decode(t, &document)
						next := document.NextAction + document.NextDetail
						if unobservable && (document.State != "unavailable" || next == "" || strings.Contains(next, "aigw provider rotate gateway")) {
							t.Errorf("%v reports the state %q and the next step %q; the store cannot observe gateway's Token, which is no missing Token", args, document.State, next)
						}
					}
					if outcomes[false] != outcomes[true] {
						t.Errorf("whether gateway's Token can be observed decides the outcome: absent %+v; unobservable %+v", outcomes[false], outcomes[true])
					}
				})
			}
		}
	}
}

// withdrawnFrom reports whether path, a file that the world's client no
// longer has, keeps neither AIGW's settings nor AIGW's record; a file that
// AIGW created is withdrawn by its removal. kept names what stayed.
func withdrawnFrom(t *testing.T, w selectionWorld, path string) (bool, string) {
	t.Helper()
	file, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	settings, record := w.keepsAIGWsPart(file), w.recordOf(t, path) != nil
	return !settings && !record, fmt.Sprintf("AIGW's settings=%t, a record=%t", settings, record)
}

// tokenStates are the three answers about the Token of a world's provider:
// usable, absent, and one that a store cannot observe.
var tokenStates = []string{"usable", "absent", "unobservable"}

// holdToken leaves the Token of the world's provider in state.
func holdToken(t *testing.T, w *selectionWorld, state string) {
	t.Helper()
	w.setToken(t, state != "absent")
	if state == "unobservable" {
		w.app.Secrets = storeThatCannotTell{Store: w.app.Secrets, accounts: []string{w.account}}
	}
}

// The twelfth review's finding 5, for Codex. Codex is on files A and B, both
// showing gateway/gpt-sol with their records, or A chose gpt-terra. The
// person runs `aigw client edit codex --target A`, which drops B, or
// `--target A --target C`, which drops B and adds C. With gateway's Token,
// without it, and with one that its store cannot observe, the edit is saved,
// AIGW's settings and record leave B, the record of each file that Codex
// keeps names what the file selects, and the stored selection is the same.
func TestAnEditThatDropsACodexFileWithdrawsWithoutTheToken(t *testing.T) {
	t.Parallel()
	for _, picked := range []bool{false, true} {
		for _, adds := range []bool{false, true} {
			t.Run(fmt.Sprintf("a choice=%t/adds a file=%t", picked, adds), func(t *testing.T) {
				outcomes := map[string]usabilityOutcome{}
				for _, state := range tokenStates {
					world := codexWorld(t, 2)
					if picked {
						world = world.picking(t, 0, basicString, "gpt-terra")
					}
					args := []string{"client", "edit", "codex", "--target", world.files[0]}
					dropped := world.files[1]
					if adds {
						fileC := filepath.Join(t.TempDir(), "config.toml")
						writeFile(t, fileC, []byte("model = \"gpt-native\"\n"), 0o600)
						args = append(args, "--target", fileC)
					}
					holdToken(t, &world, state)
					run := runIn(t, world, append(args, "--json")...)
					outcomes[state] = usabilityOutcome{refused: run.err != nil, stored: run.after.stored, targets: len(selection(t, world.app, world.client).Targets)}
					t.Logf("Token %s: err=%v\n  before: %s\n  after:  %s", state, run.err, shownLine(run.before), shownLine(run.after))
					if run.err != nil {
						t.Errorf("Token %s: the edit that drops file B was refused: %v\n%s", state, run.err, run.output)
						continue
					}
					if withdrawn, kept := withdrawnFrom(t, world, dropped); !withdrawn {
						t.Errorf("Token %s: the edit dropped file B and left %s in it", state, kept)
					}
					if !run.after.follows[0] {
						t.Errorf("Token %s: the record of file A, which Codex keeps, does not name what A selects: %s", state, shownLine(run.after))
					}
				}
				for _, state := range tokenStates[1:] {
					if outcomes[state] != outcomes["usable"] {
						t.Errorf("the Token decides the outcome of an edit that drops a file: usable %+v; %s %+v", outcomes["usable"], state, outcomes[state])
					}
				}
			})
		}
	}
}

// The twelfth review's finding 5, for Hermes. Hermes is on its one file,
// which shows dmxapi/gpt-6.2-sol with its record, or chose
// dmxapi/claude-opus-5-5 on the Anthropic protocol. The person moves Hermes
// to another file with `aigw client edit hermes --target`, which drops the
// first. With dmxapi's Token, without it, and with one that its store cannot
// observe, the edit is saved, AIGW's settings and record leave the dropped
// file, Hermes is on the new file, and the stored selection is the same.
func TestAnEditThatDropsTheHermesFileWithdrawsWithoutTheToken(t *testing.T) {
	t.Parallel()
	states := map[string]func(t *testing.T) selectionWorld{
		"recorded": hermesPlain,
		"chosen": func(t *testing.T) selectionWorld {
			return hermesPlain(t).choosing(t, "dmxapi", "anthropic", "claude-opus-5-5", plainScalars)
		},
	}
	for _, name := range slices.Sorted(mapKeys(states)) {
		t.Run(name, func(t *testing.T) {
			outcomes := map[string]usabilityOutcome{}
			for _, state := range tokenStates {
				world := states[name](t)
				dropped, moved := world.files[0], filepath.Join(t.TempDir(), "hermes", "config.yaml")
				holdToken(t, &world, state)
				run := runIn(t, world, "client", "edit", "hermes", "--target", moved, "--json")
				targets := selection(t, world.app, world.client).Targets
				outcomes[state] = usabilityOutcome{refused: run.err != nil, stored: run.after.stored, targets: len(targets)}
				t.Logf("Token %s: err=%v\n  before: %s\n  after:  %s", state, run.err, shownLine(run.before), shownLine(run.after))
				if run.err != nil {
					t.Errorf("Token %s: the edit that drops Hermes's file was refused: %v\n%s", state, run.err, run.output)
					continue
				}
				if withdrawn, kept := withdrawnFrom(t, world, dropped); !withdrawn {
					t.Errorf("Token %s: the edit dropped Hermes's file and left %s in it", state, kept)
				}
				if !slices.Equal(targets, []string{moved}) {
					t.Errorf("Token %s: Hermes is on %q, want the new file alone", state, targets)
				}
			}
			for _, state := range tokenStates[1:] {
				if outcomes[state] != outcomes["usable"] {
					t.Errorf("the Token decides the outcome of an edit that drops Hermes's file: usable %+v; %s %+v", outcomes["usable"], state, outcomes[state])
				}
			}
		})
	}
}

// `aigw sync` says that the client configuration is aligned only when no
// selected client waits for its Token: an absent Token and one that the store
// cannot observe end alike, and neither is aligned (the fresh review of
// 034296053, its first finding).
func TestSyncIsAlignedOnlyWhileNoClientWaitsForItsToken(t *testing.T) {
	t.Parallel()
	for _, state := range tokenStates {
		t.Run(state, func(t *testing.T) {
			w := codexWorld(t, 1)
			holdToken(t, &w, state)
			run := runIn(t, w, "sync")
			if run.err != nil {
				t.Fatalf("sync = %v\n%s", run.err, run.output)
			}
			aligned := strings.Contains(run.output, "Client configuration is aligned")
			waits := strings.Contains(run.output, "files wait for its Token")
			if aligned == (state != "usable") || waits == (state == "usable") {
				t.Fatalf("Token %s: aligned %t, waits %t\n%s", state, aligned, waits, run.output)
			}
		})
	}
}

// A Token file that the file store refuses for its mode names that file and
// its mode as the next step of `aigw sync` and of `aigw check --depth local`,
// never unlocking a store (the fresh review of 034296053, its second finding).
func TestARefusedTokenFileNamesItsOwnFix(t *testing.T) {
	t.Parallel()
	kind := unobservableStores()[0]
	for _, args := range [][]string{{"sync"}, {"check", "codex", "--depth", "local"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			w := codexWorld(t, 1)
			kind.hold(t, w, []string{"gateway", "spare"}, []string{"gateway"})
			run := runIn(t, w, args...)
			if !strings.Contains(run.output, "Make gateway in AIGW's Token directory") || strings.Contains(run.output, "unlocking") {
				t.Fatalf("%v names no fix of the refused Token file, or tells to unlock the store:\n%s", args, run.output)
			}
		})
	}
}
