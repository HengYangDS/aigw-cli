package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aigw-cli/internal/adapter"
	"aigw-cli/internal/domain"
)

// This file holds the eleventh review's probes of the matrix's checker
// (holds, committed, withdrew, twinned) and of the worlds' readers. Each test
// makes a wrong outcome by hand, as a product with that fault would leave it,
// and gives it to the checker through faults, which returns what the checker
// found wrong instead of reporting it. The test passes only when the checker
// names a fault: the hole is closed, and a product with that fault would now
// fail the matrix.
//
// None of these outcomes is produced by the product at this head; the
// review's sweeps (zz11_review_commands_test.go) looked for each with a
// stricter judge and found none.

// zz11MatrixCommand returns the matrix's command of that name.
func zz11MatrixCommand(t *testing.T, commands []selectionCommand, name string) selectionCommand {
	t.Helper()
	for _, command := range commands {
		if command.name == name {
			return command
		}
	}
	t.Fatalf("the matrix has no command %q", name)
	return selectionCommand{}
}

// zz11RequiresFault fails the test unless the checker names a fault that
// mentions want.
func zz11RequiresFault(t *testing.T, what string, faults []string, want string) {
	t.Helper()
	if len(faults) == 0 {
		t.Fatalf("the checker accepted %s; it must raise a fault", what)
	}
	for _, fault := range faults {
		if strings.Contains(fault, want) {
			t.Logf("the checker raised: %s", fault)
			return
		}
	}
	t.Fatalf("the checker raised %q, none of which names %q", faults, want)
}

// HOLE 1. The command "own setting" used to declare nothing that it must
// leave (selectionCommand.leaves), so a product that recorded the choice and
// dropped the edit passed. The wrong outcome is made by `aigw sync`, which
// records the choice and saves no retry budget; the checker is then asked
// about `aigw client edit codex --request-retries 2`.
func TestZZ11HoldsRejectsAnOwnSettingThatWasNotSaved(t *testing.T) {
	t.Parallel()
	world := codexOneFileMatrix().states["chosen"](t)
	before := world.snapshot(t)
	world.run(t, "sync")
	if policy := selection(t, world.app, world.client).RetryPolicy; policy != nil {
		t.Fatalf("the hand-made outcome holds a retry budget: %+v", policy)
	}
	faults := world.faults(t, before, world.snapshot(t), zz11MatrixCommand(t, codexCommands(), "own setting"), nil)
	zz11RequiresFault(t, "an edit of the client's own setting that saved no retry budget", faults, "retry budget")
}

// HOLE 2. twinned used to compare the stored model and whether each record
// follows its file, and nothing else of the client's binding. The run with
// the Token is the real command; the run without it is made by `aigw sync`,
// which saves no retry budget.
func TestZZ11TwinnedRejectsAnEditThatTheTokenDecides(t *testing.T) {
	t.Parallel()
	command := zz11MatrixCommand(t, codexCommands(), "own setting")
	build := codexOneFileMatrix().states["chosen"]
	with := selectionRun{done: true}
	world := build(t)
	with.before = world.snapshot(t)
	world.run(t, command.args...)
	with.after = world.snapshot(t)

	without := selectionRun{done: true}
	world = build(t)
	world.setToken(t, false)
	without.before = world.snapshot(t)
	world.run(t, "sync")
	without.after = world.snapshot(t)
	if with.after.binding.RetryPolicy == nil || without.after.binding.RetryPolicy != nil {
		t.Fatalf("the hand-made twins do not differ in the retry budget: %+v and %+v", with.after.binding.RetryPolicy, without.after.binding.RetryPolicy)
	}
	zz11RequiresFault(t, "twins of which one stores a retry budget and the other none", without.twinnedFaults(with, command, "gateway"), "stored binding")
}

// HOLE 3. For a preview and a refusal, holds used to compare the stored
// configuration and the client's own files and records; any other path could
// change. The wrong outcome is a file written beside the configuration, as a
// preview that wrote a backup would leave one.
func TestZZ11HoldsRejectsAPreviewThatWritesAnotherPath(t *testing.T) {
	t.Parallel()
	world := codexOneFileMatrix().states["chosen"](t)
	before := world.snapshot(t)
	stray := filepath.Join(filepath.Dir(world.app.Config.Path()), "written-by-a-preview")
	writeFile(t, stray, []byte("anything"), 0o600)
	faults := world.faults(t, before, world.snapshot(t), zz11MatrixCommand(t, codexCommands(), "sync preview"), nil)
	zz11RequiresFault(t, "a preview after which a path beside the configuration exists", faults, "a path other than")
}

// HOLE 4. The Codex world used to read the stored selection as its model ID
// alone and the record as projected_model alone. A stored selection of
// another provider's model with the same ID counted as the model that the
// file chose, although Codex's file still reaches the first provider's
// endpoint. The wrong outcome: the person picked gpt-terra in a file written
// for gateway; by hand spare/gpt-terra is stored and the record names
// gpt-terra.
func TestZZ11HoldsRejectsAChoiceStoredForAnotherProvider(t *testing.T) {
	t.Parallel()
	world := codexOneFileMatrix().states["chosen"](t)
	before := world.snapshot(t)
	cfg, err := world.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	other := domain.NewModelRef("spare", "gpt-terra")
	cfg.Models[other] = qualifiedModel("Spare Terra", "spare", "gpt-terra", domain.ProtocolResponses)
	cfg.SetSelectedModel(world.client, other, "")
	if err := world.app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	record := world.files[0] + adapter.StateSuffix
	var state map[string]any
	if err := json.Unmarshal(readFile(t, record), &state); err != nil {
		t.Fatal(err)
	}
	state["projected_model"] = "gpt-terra"
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, record, data, 0o600)
	if got := selection(t, world.app, world.client).Model.String(); got != "spare/gpt-terra" || !strings.Contains(string(readFile(t, world.files[0])), "https://gateway.test/v1") {
		t.Fatalf("the hand-made outcome is not the wrong one: %s is stored", got)
	}
	faults := world.faults(t, before, world.snapshot(t), zz11MatrixCommand(t, codexCommands(), "another provider"), nil)
	zz11RequiresFault(t, "spare/gpt-terra stored for a file that chose gpt-terra and reaches gateway", faults, "an endpoint of another provider")
}

// HOLE 5. committed used to pass over a file that is gone after the command,
// in every state. The wrong outcome: the file is removed by hand and nothing
// else changes.
func TestZZ11HoldsRejectsACommandThatRemovesTheFile(t *testing.T) {
	t.Parallel()
	world := codexOneFileMatrix().states["chosen"](t)
	before := world.snapshot(t)
	if err := os.Remove(world.files[0]); err != nil {
		t.Fatal(err)
	}
	faults := world.faults(t, before, world.snapshot(t), zz11MatrixCommand(t, codexCommands(), "another provider"), nil)
	zz11RequiresFault(t, "a committed command after which Codex's one file is gone", faults, "removed file")
}

// HOLE 6. The check of a rollback used to be the stored model alone, and in
// the states whose last change did not select a model the restored model is
// the stored one, so the check held whatever the rollback did. In the Hermes
// state "forwards, chosen of another provider" the last change set a
// forwarding, which a rollback must end: the wrong outcome keeps it. Here the
// forwarding edit itself is the last change, and a "rollback" that restores
// nothing of it is made by `aigw use hermes` of the stored model.
func TestZZ11HoldsRejectsARollbackThatKeepsWhatTheLastChangeSet(t *testing.T) {
	t.Parallel()
	world := hermesForwarding(t)
	before := world.snapshot(t)
	if before.binding.ForwardingEndpoint == "" {
		t.Fatal("the state forwards nowhere")
	}
	// The binding that the last change replaced, which a rollback restores.
	restored := before.binding
	restored.ForwardingEndpoint, restored.ForwardingUpstreamIdentity, restored.ForwardingProtocol = "", "", ""
	world.restoredBinding = &restored
	world.run(t, "use", "hermes", "dmxapi/gpt-6.2-sol", "--protocol", "responses")
	if after := selection(t, world.app, world.client); after.ForwardingEndpoint == "" {
		t.Fatal("the hand-made outcome ended the forwarding")
	}
	faults := world.faults(t, before, world.snapshot(t), zz11MatrixCommand(t, hermesCommands(), "rollback"), nil)
	zz11RequiresFault(t, "a rollback after which the forwarding that the last change set is still stored", faults, "not the restored")
}

// HOLE 7. withdrew used to read the files and the records, never the stored
// configuration: a withdrawal after which the client is still enabled passed.
// The wrong outcome is a real `aigw client disable`, after which the client is
// enabled again by hand in the stored configuration.
func TestZZ11HoldsRejectsAWithdrawalThatLeavesTheClientEnabled(t *testing.T) {
	t.Parallel()
	world := codexOneFileMatrix().states["chosen"](t)
	before := world.snapshot(t)
	world.run(t, "client", "disable", "codex")
	cfg, err := world.app.Config.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := cfg.Clients[world.client]
	binding.Enabled = true
	cfg.Clients[world.client] = binding
	if err := world.app.Config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	faults := world.faults(t, before, world.snapshot(t), zz11MatrixCommand(t, codexCommands(), "disable"), nil)
	zz11RequiresFault(t, "a withdrawal after which Codex is enabled in the stored configuration", faults, "enabled")
}

// A declaration that the matrix relies on, checked against the product: the
// state "recorded, the other file missing" is declared blocked, and the
// commands that name Codex are refused in it with the Token and without. The
// probe only shows what the matrix's escape "blocked and untouched" covers
// there: every command of the matrix that succeeds leaves Codex untouched.
func TestZZ11WhatTheBlockedStatesLetThrough(t *testing.T) {
	t.Parallel()
	for name, build := range codexTwoFilesMatrix().states {
		declared := build(t)
		if declared.blocked == "" {
			continue
		}
		for _, command := range codexCommands() {
			if command.preview || command.withdraws || (command.where != nil && !command.where(declared)) {
				continue
			}
			world := build(t)
			run := runIn(t, world, command.args...)
			if run.err == nil && run.after.clientUntouched(run.before) {
				t.Logf("%s / %s: succeeded and left Codex as it was (blocked: %s); it changed %v", name, command.name, declared.blocked, run.changed)
			}
		}
	}
}
