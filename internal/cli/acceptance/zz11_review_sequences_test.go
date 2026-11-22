package cli_test

import (
	"aigw-cli/internal/adapter/codex"
	"aigw-cli/internal/adapter/discovery"
	"aigw-cli/internal/cli"
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// This file holds the eleventh review's probes of single sequences that the
// sweeps of zz11_review_commands_test.go do not run: guided setup, rollbacks
// over a choice, edits of the same files, a Token that cannot be read, a
// verified snapshot, a new credential reader, and enabling an enabled client.

// Guided setup on a configured home after a choice. `aigw setup` at a
// terminal offers the provider and asks for a model, which it selects for the
// clients that speak the provider's protocol: an explicit selection made
// through prompts.
//
// Before: F = gpt-terra (the person's pick in Codex), R = gpt-sol,
// S = gateway/gpt-sol. The person runs `aigw setup`, chooses the provider
// gateway and names a model: the stored one, the chosen one, or a third.
//
// The rule: after a setup that succeeds, the record names what the file
// selects; the model that the person named in the setup is stored, as after
// `aigw use`; and the Token decides neither.
func TestZZ11GuidedSetupAfterAChoice(t *testing.T) {
	for _, named := range []string{"gpt-sol", "gpt-terra", "gpt-luna"} {
		outcomes := map[bool]string{}
		for _, token := range []bool{true, false} {
			t.Run(fmt.Sprintf("names %s/token=%t", named, token), func(t *testing.T) {
				world := zz11Codex(codexWorld(t, 1)).picking(t, 0, basicString, "gpt-terra")
				world.setToken(t, token)
				binding := selection(t, world.app, world.client)
				world.app.Interactive = true
				world.app.Discovery = fakeDiscovery{result: discovery.Result{
					Executables: map[string]string{codex.ID: binding.Executable},
					Surfaces:    []discovery.Surface{{ID: string(codex.HomeDefault), ConfigPath: world.files[0], Present: true, AutoManaged: true}},
				}}
				prompt := &scriptedPrompt{selections: []string{"gateway"}, texts: []string{named}, secrets: []string{"typed-token"}}
				world.app.Prompt = prompt
				run := runIn(t, world, "setup")
				t.Logf("err=%v; prompts: %d selections, %d texts, secrets %q\n  before: %s\n  after:  %s\n%s", run.err, prompt.selectCalls, prompt.textCalls, prompt.secretCalls,
					shownLine(run.before), shownLine(run.after), run.output)
				outcomes[token] = fmt.Sprintf("refused=%t stored=%s behind=%v", run.err != nil, run.after.stored, !run.after.follows[0])
				if run.err != nil {
					if len(run.changed) != 0 {
						t.Errorf("the setup was refused and changed %v", run.changed)
					}
					return
				}
				if !run.after.follows[0] {
					t.Errorf("the setup succeeded and left the record behind the file: %s", shownLine(run.after))
				}
				if run.after.stored != named {
					t.Errorf("the person named %s in the setup, and %s is stored", named, run.after.stored)
				}
			})
		}
		if outcomes[true] != outcomes[false] {
			t.Logf("names %s: with the Token %s; without it %s", named, outcomes[true], outcomes[false])
		}
	}
}

// A rollback whose restored configuration holds the client on other files
// than the stored one, with a choice and without one, with the Token and
// without it.
//
// Before: Codex was on file A. `aigw client edit codex --target B` moved it to
// file B: A holds nothing of AIGW's, B shows gpt-sol with a record, and
// S = gateway/gpt-sol. With a choice, the person then selects gpt-terra in B
// (F(B) = gpt-terra, R(B) = gpt-sol). Then `aigw config rollback
// --last-change` runs, which restores Codex on file A.
//
// The rule: when the rollback succeeds, every file that holds AIGW's settings
// has a record that names what the file selects, and no file that the
// restored configuration does not name keeps AIGW's settings; or it is
// refused and changes nothing. The Token decides neither the refusal nor what
// is stored, and once the Token is stored a synchronization reaches the state
// that the rollback reaches with the Token.
//
// The case without a choice shows what the choice adds: without one the file
// that Codex left keeps AIGW's settings too, and its record is not written.
func TestZZ11ARollbackToOtherFilesOverAChoice(t *testing.T) {
	type outcome struct {
		refused bool
		stored  string
	}
	for _, picked := range []bool{true, false} {
		outcomes := map[bool]outcome{}
		for _, token := range []bool{true, false} {
			t.Run(fmt.Sprintf("a choice=%t/token=%t", picked, token), func(t *testing.T) {
				world := zz11Codex(codexWorld(t, 1))
				fileA := world.files[0]
				fileB := filepath.Join(t.TempDir(), "config.toml")
				writeFile(t, fileB, []byte("model = \"gpt-native\"\n"), 0o600)
				world.run(t, "client", "edit", "codex", "--target", fileB)
				world.files = []string{fileA, fileB}
				if picked {
					recPick(t, fileB, "gpt-terra")
				}
				world.setToken(t, token)
				run := runIn(t, world, "config", "rollback", "--last-change", "--json")
				binding := selection(t, world.app, world.client)
				partA, partB := world.keepsAIGWsPart(run.after.files[0]), world.keepsAIGWsPart(run.after.files[1])
				recordB := run.after.records[1] != nil
				outcomes[token] = outcome{refused: run.err != nil, stored: run.after.stored}
				t.Logf("err=%v\n  before: %s\n  after:  %s\n  Codex is on file A: %t; AIGW's settings in A: %t, in B: %t; a record of A: %t, of B: %t; the record of B was written: %t\n%s",
					run.err, shownLine(run.before), shownLine(run.after), slices.Equal(binding.Targets, []string{fileA}), partA, partB,
					run.after.records[0] != nil, recordB, !bytes.Equal(run.before.records[1], run.after.records[1]), run.output)
				if run.err != nil {
					if len(run.changed) != 0 {
						t.Errorf("the rollback was refused and changed %v", run.changed)
					}
					return
				}
				if !slices.Equal(binding.Targets, []string{fileA}) {
					t.Errorf("the rollback succeeded and Codex is on %v, not on file A", binding.Targets)
				}
				if partB || recordB {
					t.Errorf("the rollback succeeded, Codex is no longer on file B, and file B keeps AIGW's settings (%t) or its record (%t), which no later command visits", partB, recordB)
				}
				if partA && !run.after.follows[0] {
					t.Errorf("the rollback succeeded and the record of file A does not name what the file selects: %s", shownLine(run.after))
				}
				if token {
					return
				}
				// The Token arrives, and a synchronization writes what waited.
				world.setToken(t, true)
				sync := runIn(t, world, "sync", "--json")
				t.Logf("then the Token and a synchronization: err=%v\n  after: %s\n  AIGW's settings in B: %t; a record of B: %t", sync.err, shownLine(sync.after),
					world.keepsAIGWsPart(sync.after.files[1]), sync.after.records[1] != nil)
				if world.keepsAIGWsPart(sync.after.files[1]) || sync.after.records[1] != nil {
					t.Errorf("after the Token and a synchronization, file B, which Codex is no longer on, still keeps AIGW's settings or its record")
				}
			})
		}
		if with, without := outcomes[true], outcomes[false]; with != without {
			t.Errorf("a choice=%t: the Token decides the outcome: with it refused=%t, %q stored; without it refused=%t, %q stored",
				picked, with.refused, with.stored, without.refused, without.stored)
		}
	}
}

// A rollback whose restored configuration holds the client disabled, over a
// choice: the last change enabled Codex again, the person then selected
// gpt-terra in its file, and `aigw config rollback --last-change` restores the
// disabled client.
//
// The rule: when the rollback succeeds it withdraws as `aigw client disable`
// does (no record, none of AIGW's settings in the file), or it is refused and
// changes nothing; alike with the Token and without it.
func TestZZ11ARollbackToADisabledClientOverAChoice(t *testing.T) {
	outcomes := map[bool]string{}
	for _, token := range []bool{true, false} {
		t.Run(fmt.Sprintf("token=%t", token), func(t *testing.T) {
			world := zz11Codex(codexWorld(t, 1))
			binding := selection(t, world.app, world.client)
			world.run(t, "client", "disable", "codex")
			world.run(t, "client", "enable", "codex", "--executable", binding.Executable, "--target", world.files[0])
			recPick(t, world.files[0], "gpt-terra")
			world.setToken(t, token)
			run := runIn(t, world, "config", "rollback", "--last-change", "--json")
			after := selection(t, world.app, world.client)
			part := world.keepsAIGWsPart(run.after.files[0])
			outcomes[token] = fmt.Sprintf("refused=%t enabled=%t settings=%t record=%t", run.err != nil, after.Enabled, part, run.after.records[0] != nil)
			t.Logf("err=%v; enabled %t; AIGW's settings in the file %t; a record %t\n  before: %s\n  after:  %s\n%s", run.err, after.Enabled, part, run.after.records[0] != nil,
				shownLine(run.before), shownLine(run.after), run.output)
			if run.err != nil {
				if len(run.changed) != 0 {
					t.Errorf("the rollback was refused and changed %v", run.changed)
				}
				return
			}
			if after.Enabled || part || run.after.records[0] != nil {
				t.Errorf("the rollback succeeded and did not withdraw Codex: enabled %t, AIGW's settings %t, a record %t", after.Enabled, part, run.after.records[0] != nil)
			}
		})
	}
	if outcomes[true] != outcomes[false] {
		t.Errorf("the Token decides the outcome: with it %s; without it %s", outcomes[true], outcomes[false])
	}
}

// `aigw client edit <client> --target` with the files that the client has
// already: an edit that asks for what is stored, with the Token and without.
//
// The Change says that an edit which asks for what is stored reports that
// nothing changed, and the rule says that what a command refuses does not
// depend on a Token. Before: every file shows its recorded model (F = R = S),
// or one file chose (F = gpt-terra, R = gpt-sol, S = gateway/gpt-sol).
func TestZZ11TheSameFilesAreRefusedWithoutTheToken(t *testing.T) {
	for name, build := range map[string]func(t *testing.T) selectionWorld{
		"recorded": func(t *testing.T) selectionWorld { return codexWorld(t, 1) },
		"chosen":   func(t *testing.T) selectionWorld { return codexWorld(t, 1).picking(t, 0, basicString, "gpt-terra") },
	} {
		t.Run(name, func(t *testing.T) {
			refused := map[bool]error{}
			stored := map[bool]string{}
			for _, token := range []bool{true, false} {
				world := build(t)
				world.setToken(t, token)
				run := runIn(t, world, zz11SameTargets(t, world)...)
				refused[token], stored[token] = run.err, run.after.stored
				t.Logf("token=%t: err=%v\n  before: %s\n  after:  %s", token, run.err, shownLine(run.before), shownLine(run.after))
			}
			if (refused[true] == nil) != (refused[false] == nil) || stored[true] != stored[false] {
				t.Errorf("the Token decides the outcome of an edit to the files that the client has: with it err=%v and %q stored; without it err=%v and %q stored",
					refused[true], stored[true], refused[false], stored[false])
			}
		})
	}
}

// zz11DeniedStore answers that every Token is there and refuses to read one,
// as a locked keychain does, or refuses to say whether one is there.
func zz11DeniedStore(t *testing.T, w selectionWorld, exists bool) {
	t.Helper()
	denied := errors.New("the keychain is locked")
	store := &recordingCredentialStore[string]{backend: w.app.Secrets, getErr: denied}
	if exists {
		store.existsErr = denied
	}
	w.app.Secrets = store
}

// A Token that is there and cannot be read, or whose presence cannot be
// observed: the third way in which a provider has no Token that AIGW can use,
// beside an absent and an empty one.
//
// Before: F = gpt-terra, R = gpt-sol, S = gateway/gpt-sol. The rule: a command
// records the choice or is refused and changes nothing.
func TestZZ11AChoiceWhileTheTokenCannotBeRead(t *testing.T) {
	for _, exists := range []bool{false, true} {
		for _, name := range slices.Sorted(mapKeys(zz11Commands())) {
			t.Run(fmt.Sprintf("presence refused=%t/%s", exists, name), func(t *testing.T) {
				world := zz11Codex(codexWorld(t, 1)).picking(t, 0, basicString, "gpt-terra")
				zz11DeniedStore(t, world, exists)
				run := runIn(t, world, zz11Commands()[name]...)
				class, faults := zz11Did(run)
				t.Logf("%s (err=%v)\n  before: %s\n  after:  %s", class, run.err, shownLine(run.before), shownLine(run.after))
				for _, fault := range faults {
					t.Error(fault)
				}
				if class == "left" || class == "broken" {
					t.Errorf("%s succeeded and left the record behind the pick: %s", name, shownLine(run.after))
				}
			})
		}
	}
}

// A rollback to the verified snapshot that a check wrote while a choice was
// not yet recorded. `aigw check --depth client` passes with the model that
// Codex's file chose and writes the verified snapshot of the configuration
// that it read, which names the choice; the stored configuration still names
// the earlier model.
//
// Before the rollback: F = gpt-terra, R = gpt-sol, S = gateway/gpt-sol, and
// the verified snapshot names gateway/gpt-terra. `aigw config rollback` then
// restores the snapshot, with the Token and without it. The rule: it stores
// gateway/gpt-terra and the record names gpt-terra.
func TestZZ11ARollbackToASnapshotThatACheckWroteOverAChoice(t *testing.T) {
	for _, token := range []bool{true, false} {
		t.Run(fmt.Sprintf("token=%t", token), func(t *testing.T) {
			world := zz11Codex(codexWorld(t, 1)).picking(t, 0, basicString, "gpt-terra")
			check := runIn(t, world, "check", "--depth", "client", "--json")
			if check.err != nil {
				t.Fatalf("the check of every client: %v\n%s", check.err, check.output)
			}
			t.Logf("the check changed %v", check.changed)
			if !check.after.clientUntouched(check.before) || check.after.stored != "gpt-sol" {
				t.Errorf("the check changed the client or the stored selection: %s", shownLine(check.after))
			}
			snapshot, err := world.app.Config.LoadVerifiedCheckpoint()
			if err != nil {
				t.Fatalf("no verified snapshot: %v", err)
			}
			t.Logf("the verified snapshot names %s for Codex", snapshot.Config.Clients[world.client].Model)
			world.setToken(t, token)
			run := runIn(t, world, "config", "rollback", "--json")
			t.Logf("rollback: err=%v\n  before: %s\n  after:  %s\n%s", run.err, shownLine(run.before), shownLine(run.after), run.output)
			if run.err != nil {
				if len(run.changed) != 0 {
					t.Errorf("the rollback was refused and changed %v", run.changed)
				}
				return
			}
			if !settledOn(run.after, "gpt-terra") {
				t.Errorf("the rollback to the snapshot left: %s", shownLine(run.after))
			}
		})
	}
}

// A new credential reader beside a choice. A new program keeps its reader at
// another path, which every client file that AIGW wrote names; the probe
// imitates it by binding a new reader to the same application (no program is
// installed or run). The next command must still record the choice, with the
// Token and without it.
//
// Before: F = gpt-terra, R = gpt-sol, S = gateway/gpt-sol, and Codex's file
// names the earlier reader.
func TestZZ11AChoiceBesideANewCredentialReader(t *testing.T) {
	for _, name := range slices.Sorted(mapKeys(zz11Commands())) {
		classes := map[bool]string{}
		for _, token := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/token=%t", name, token), func(t *testing.T) {
				world := zz11Codex(codexWorld(t, 1)).picking(t, 0, basicString, "gpt-terra")
				world.setToken(t, token)
				bindCredentialReader(t, world.app)
				// The reader's folder is outside the world's root, so the run is
				// read without the tree.
				before := world.snapshot(t)
				world.output.Reset()
				err := cli.Execute(world.app, zz11Commands()[name])
				run := ranIn{err: err, before: before, after: world.snapshot(t), output: world.output.String()}
				class, faults := zz11Did(run)
				classes[token] = class
				t.Logf("%s (err=%v)\n  before: %s\n  after:  %s", class, run.err, shownLine(run.before), shownLine(run.after))
				for _, fault := range faults {
					t.Error(fault)
				}
				if class == "left" || class == "broken" {
					t.Errorf("%s succeeded and left the record behind the pick: %s", name, shownLine(run.after))
				}
			})
		}
		if classes[true] != classes[false] {
			t.Errorf("%s: the Token decides the outcome: %q with it, %q without it", name, classes[true], classes[false])
		}
	}
}

// `aigw client enable` for a client that is enabled, on the files that it has
// and on another file, after a choice, with the Token and without it. The
// probe shows what the command does; it fails only where a success leaves the
// file that the client is no longer on with AIGW's settings.
func TestZZ11EnablingAnEnabledClientAfterAChoice(t *testing.T) {
	for _, other := range []bool{false, true} {
		for _, token := range []bool{true, false} {
			t.Run(fmt.Sprintf("another file=%t/token=%t", other, token), func(t *testing.T) {
				world := zz11Codex(codexWorld(t, 1)).picking(t, 0, basicString, "gpt-terra")
				binding := selection(t, world.app, world.client)
				target := world.files[0]
				if other {
					target = filepath.Join(t.TempDir(), "config.toml")
					writeFile(t, target, []byte("model = \"gpt-native\"\n"), 0o600)
					world.files = append(world.files, target)
				}
				world.setToken(t, token)
				run := runIn(t, world, "client", "enable", "codex", "--executable", binding.Executable, "--target", target, "--json")
				t.Logf("err=%v\n  before: %s\n  after:  %s", run.err, shownLine(run.before), shownLine(run.after))
				if run.err != nil {
					if len(run.changed) != 0 {
						t.Errorf("the command was refused and changed %v", run.changed)
					}
					return
				}
				if on := selection(t, world.app, world.client).Targets; other && !slices.Contains(on, world.files[0]) && world.keepsAIGWsPart(run.after.files[0]) {
					t.Errorf("Codex is now on %v, and the file that it was on keeps AIGW's settings", on)
				}
			})
		}
	}
}
