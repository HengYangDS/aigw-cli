## Context

A client's model can differ from AIGW's record in these ways, which the owner and I listed on 2026-11-20:

| How the two came to differ                                      | What the person wants | Who wins          |
| --------------------------------------------------------------- | --------------------- | ----------------- |
| The person chose another added model in Codex or Hermes         | that model            | the client        |
| The person chose a model in Claude Code                         | that model            | the client        |
| The person chose a model in a Claude Desktop conversation       | that model, there     | the client        |
| The person ran `aigw use`                                       | the named model       | AIGW              |
| The client's file names a model that AIGW has not added         | not known             | neither: reported |
| After `aigw use`, a running client wrote its earlier model back | the named model       | not told apart    |

The last row cannot be told from the first by reading the file, so it is followed as the first is; the owner accepted
that cost on 2026-11-20.

Release 0.8.3 follows the first row in `aigw sync` alone. Every command that writes client files ends in one
transaction (`Synchronizer.commit`) and every preview in one planner; each adapter's plan refuses a file whose model
equals neither AIGW's record nor the new configuration's model. So a command other than sync refuses until sync has
run.

A first version of this Change followed the choice inside the transaction. A fresh review ran it and showed that the
transaction then saves a configuration that the command never saw: `aigw client edit hermes --via` lost its own edit
to the follow, results printed the unfollowed configuration, and a follow could be saved without a line anywhere.
The decisions below replace that version.

Measured on 2026-11-20 with the installed 0.8.3 in an isolated home, the clients' writes imitated by changing the one
value in each file: the behaviors that the proposal lists. Not measured: what a real Codex or Hermes writes when a
model is chosen in it; the adoption of 2026-10-16 for Hermes followed a file that the owner's own Hermes had written,
and the one for Codex of 2026-10-19 rests on an imitation. The next Change in this lane measures both.

## Goals / Non-Goals

**Goals:**

- A model chosen in Codex or Hermes survives every AIGW command and is reported as the client's selection.
- A value that AIGW cannot follow always has one step that restores a working state.
- One owner decides what a client's own selection means, one owner reads the configuration for commands, and one
  owner reports what was followed.

**Non-Goals:**

- Recording a model that Claude Code or Claude Desktop chose: AIGW cannot map an alias such as `opus[1m]` to a model
  of a provider, and a Desktop conversation's model is not in a file that AIGW owns.
- Telling a stale write of a running client from a choice.
- Any sentence about what a running client does after `aigw use`; it waits for the measurement.

## Decisions

### Commands read a working configuration that follows the clients

- **Rests on:** every command loaded the stored configuration itself, at 26 places, and built its change and its
  result from it. Tests run a command that changes the client's own settings, one that writes none of its files, an
  explicit selection and a withdrawal after a choice, and read the client's file, the saved configuration and the
  result; one test refuses a direct read of the stored configuration anywhere in the command tree.
- **Considered:** follow inside the transaction, after the command built its change; follow inside each adapter's
  plan; follow where the configuration is read.
- **Chosen because:** only a follow before the command's own change lets that change, its validation and its result
  see one configuration. The transaction cannot be the place, as the first version showed, and an adapter's plan
  cannot change the configuration.
- **Applies to:** `Dependencies.WorkingConfiguration` and `LoadConfiguration` in `internal/app`, and
  `Synchronizer.Working` in `internal/app/synchronization`. A choice that AIGW does not follow, a file that it cannot
  read, or a followed selection that would not validate leaves that client as stored, so a command about something
  else is not stopped by a client's file.

### The transaction's preimage is the working configuration

- **Rests on:** `commit` refuses when the stored configuration no longer matches what the command read; a command
  now reads the working configuration, which differs from the stored one after a choice.
- **Considered:** pass the stored configuration beside the working one through every command; compare the
  transaction's preimage with the same view of the stored configuration.
- **Chosen because:** the view is one function, and a client's file that changes between the read and the commit
  then fails the same guard as a configuration that changed.
- **Applies to:** `store.Snapshot.MatchesConfiguration`, which takes the view, and `commit`.

### An adapter admits a file that shows the selection the transition starts from

- **Rests on:** a transition now starts from a configuration that names the client's choice while AIGW's record of
  the file still names the earlier model. A table test runs every state of Hermes's model block (a listed model, an
  unlisted one, another entry's model with the earlier address, an edited address, a removed mode or model, a
  provider that AIGW does not own, no block, a block that is one word, a listed model beside an edited entry)
  against following, an explicit selection, a transition and a withdrawal; Codex's ownership tests run the root
  model against the recorded, the new and the starting model.
- **Considered:** rewrite AIGW's record of the file when the choice is observed; admit the starting selection in the
  ownership guard.
- **Chosen because:** observation writes nothing, so that reports stay read-only; the guard already admits the model
  about to be written, and the starting model is the same kind of proof. Hermes's block is classified once: a listed
  model of an owned entry is a choice; an unlisted model or a provider that AIGW does not own is a selection that
  AIGW does not follow; an address or mode that differs from the entry is an edit of an endpoint and stays an
  outside edit, for an explicit selection too.
- **Applies to:** `removeCodexProjectionTransition` and Codex's inspection; `classifySelection`,
  `ObserveSelection` and `PrepareTransition` in `internal/adapter/hermes`, which replace the adopted and authorized
  variants.

### A selection that AIGW does not follow is a typed refusal with a step that works

- **Rests on:** the run of 2026-11-20: for Codex the Fix, parsed from the error's text, was
  `aigw use codex team/<the unknown ID>`, which fails when the provider does not serve it and which followed the
  foreign value on 2026-11-19; for Hermes the Fix was "Undo the outside edit", and `aigw use hermes` refused too. A
  script runs the Fix as printed and then `aigw sync`.
- **Considered:** keep the Fix that adds the named model; overwrite the value with the recorded model; refuse with
  both commands, the restoring one as the Fix.
- **Chosen because:** only the restoring command runs in every case, and a Fix must run; overwriting would discard a
  model that the person typed on purpose. A model ID that a shell would not pass through unchanged gives the step in
  prose, by the rule that every other command line follows.
- **Applies to:** `SelectionError`, `RecordedSelection` and `SelectCommand` in `internal/adapter`; the registry
  replaces a plan's conflict, and an inspection's issue, by that refusal when the file's only difference is such a
  selection, so that `sync`, another command, a withdrawal, `status` and `check` name one step. The shell-word rule
  moves from the command layer to `internal/platform`, where the adapters can reach it.

### The invocation reports what it followed, once

- **Rests on:** the first version put the report into the client-file rows, and the review listed the commands
  whose results never pass through those rows (setup, rollback, enable, sync's applied result) or state the opposite
  beside them. Tests read the report of a saving command, a preview, an explicit selection, `status`, `check` and
  bare `aigw`, and of a command that reads without reporting selections.
- **Considered:** a field and a line in each command's own result; the client-file rows; one report by the owner of
  the invocation's output.
- **Chosen because:** what was followed is a fact of the invocation, not of one command's result, and the root
  already owns the output of every command. It reads the choices before the command runs and reports, after a
  success, those that the saved configuration then records; a preview and a report name all of them.
- **Applies to:** `followedReport` in `internal/cli`, which declares the saving and the reporting commands once (the
  configuration lock reads the same declaration), and `terminal.WriteDocument`, which adds the fields that the
  invocation's output owns. `aigw sync` loses `adopted_selections`.

### Claude Code's own model is shown, not recorded

- **Rests on:** `InspectSettings` already recognises a settings file whose only difference is its `model`, and
  returned a boolean; the value is in scope there. Tests read the status row and both documents.
- **Considered:** record the value as the selection; show it beside AIGW's selection.
- **Chosen because:** the value may be an alias that names no model of a provider, so it cannot be a selection;
  showing it tells the person which model Claude Code sends.
- **Applies to:** `internal/adapter/claude`, `adapter.Status` with the one wording of the note, the status row and
  the check's detail, and `client_model` in both documents.

## Risks / Trade-offs

- A stale write of a running client is followed as a choice. The followed model, the earlier one and the returning
  command are printed, and the next Change words what a running client needs.
- Every command now reads the enabled clients' files before it runs. A file that cannot be read leaves its client as
  stored and costs one read.
- The clients' writes are imitated in every test of this Change; the next Change's measurement with the real
  clients may change the conditions of following.

## Migration Plan

No data changes. A script reads `followed_selections` where it read `adopted_selections`, and receives the restoring
command as the Fix of a selection that AIGW does not follow.
