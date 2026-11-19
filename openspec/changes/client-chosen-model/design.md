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

Release 0.8.3 follows the first row in `aigw sync` alone (`Registry.AdoptSelections`, called from the sync command).
Every command that writes client files ends in one transaction (`Synchronizer.commit`), and every preview in one
planner (`project`); each adapter's plan refuses a file whose model equals neither AIGW's record nor the new
configuration's model. So a command other than sync refuses until sync has run.

Measured on 2026-11-20 with the installed 0.8.3 in an isolated home, the clients' writes imitated by changing the one
value in each file: the behaviors that the proposal lists. Not measured: what a real Codex or Hermes writes when a
model is chosen in it; the adoption of 2026-10-16 for Hermes followed a file that the owner's own Hermes had written,
and the one for Codex of 2026-10-19 rests on an imitation. The next Change in this lane measures both.

## Goals / Non-Goals

**Goals:**

- A model chosen in Codex or Hermes survives every AIGW command and is reported as the client's selection.
- A value that AIGW cannot follow always has one command that runs and restores a working state.
- One owner decides what a client's own selection means; commands do not each decide.

**Non-Goals:**

- Recording a model that Claude Code or Claude Desktop chose: AIGW cannot map an alias such as `opus[1m]` to a model
  of a provider, and a Desktop conversation's model is not in a file that AIGW owns.
- Telling a stale write of a running client from a choice.
- Any sentence about what a running client does after `aigw use`; it waits for the measurement.

## Decisions

### The transaction and the planner follow a client's choice, not each command

- **Rests on:** every writer ends in `Synchronizer.commit` and every preview in `project`; a test for each of the
  commands that write client files runs it after a model was chosen in the client and reads the client's file, the
  configuration and the result.
- **Considered:** follow when a command loads the configuration; follow inside each adapter's plan; follow in the
  transaction and the planner.
- **Chosen because:** a command keeps one configuration value for "before" and "after" today, and following at load
  would make every command carry two; an adapter's plan cannot change the configuration. The transaction already
  holds both values and the set of clients that it writes, so one step there serves every command, and the planner
  shares the step so that a preview shows what the run does.
- **Applies to:** `commit` and `project` in `internal/app/synchronization`, through one function that both call.

### Only the clients that the transition writes are followed, and never the client that it selects for

- **Rests on:** `Commit` reconciles the clients whose configuration changed; a test shows a command that changes
  nothing of Codex leaving Codex's file and record alone, and `aigw use` keeping its named model.
- **Considered:** follow every enabled client in every command; follow only the clients that the transition writes.
- **Chosen because:** following a client makes the transition write that client's files, so following all of them
  would let a command about one provider fail on another client's file. `aigw status` reports the chosen model from
  the file, so the record's delay until the next write is not visible to the person. A transition that itself
  changes the client's selection is an explicit choice and wins.
- **Applies to:** the step above; a client whose binding the transition changed, or for which the selection is
  authorized, is skipped.

### A model that AIGW has not added is a typed refusal with a Fix that runs

- **Rests on:** the run of 2026-11-20: for Codex the Fix, parsed from the error's text, was
  `aigw use codex team/<the unknown ID>`, which fails when the provider does not serve it and which followed the
  foreign value on 2026-11-19; for Hermes the Fix was "Undo the outside edit", and `aigw use hermes` refused too.
- **Considered:** keep the Fix that adds the named model; overwrite the value with the recorded model; refuse with
  both commands, the restoring one as the Fix.
- **Chosen because:** only the restoring command runs in every case, and a Fix must run; overwriting would discard a
  model that the person typed on purpose. The adding command is named beside it, because that is the other thing the
  person may have meant.
- **Applies to:** one error type in `internal/adapter`, which carries the client, the named model, the recorded
  selection and the adding command, and which `terminal` reads for its next action; `modelFor` and `hermesModelFor`
  return it, and Hermes's observation returns it when only the selection block differs from the record.

### An explicit selection for Hermes needs only that nothing else changed

- **Rests on:** `selectionOnlyDrift` proves that only the selection moved by replacing it with an owned provider and
  model and comparing the digest, and before that it requires the file's own selection to be a listed model. A test
  writes an unlisted model into the file and runs `aigw use hermes`.
- **Considered:** leave the requirement; drop it for an authorized selection.
- **Chosen because:** the replacement proof alone shows that nothing else changed, which is all that an explicit
  selection needs before it writes the block; the listed-model requirement is what following needs.
- **Applies to:** `PrepareAuthorizedSelection` and `selectionOnlyDrift` in `internal/adapter/hermes`.

### Status and check read the file's choice through the same observation

- **Rests on:** Codex's inspection compares the file with the configuration's model and calls every difference
  drift; Hermes's inspection already asks whether synchronization would adopt. A test for each client reads the row
  after a choice.
- **Considered:** a second classifier in the status code; the adapters' own observation, applied to a copy of the
  configuration before inspection.
- **Chosen because:** one observation decides what counts as a choice for the transaction and for the reports, so
  the two cannot disagree. The reports write nothing.
- **Applies to:** `internal/app/status.go` and the local check, which inspect against the followed configuration and
  mark the row; a refusal of the observation becomes the row's issue and next action.

### The followed model is reported in the client-file row

- **Rests on:** every command that writes client files names them through `invocation.ClientFiles` and
  `RenderClientFiles`; `aigw sync` has its own "Adopted selection" line.
- **Considered:** a field in each command's document; a line that the common output adds; the client-file row.
- **Chosen because:** the row exists in every such result and preview and names the client's file, where the model
  was chosen; one renderer words it once. `aigw sync` keeps its `adopted_selections` field, which scripts read, and
  takes its line from the same source.
- **Applies to:** `Projection` in `internal/app/synchronization`, `invocation.ClientFile` and its renderer, and the
  sync command.

### Claude Code's own model is shown, not recorded

- **Rests on:** `InspectSettings` already recognises a settings file whose only difference is its `model`, and
  returns a boolean; the value is in scope there. A test reads the status row and document.
- **Considered:** record the value as the selection; show it beside AIGW's selection.
- **Chosen because:** the value may be an alias that names no model of a provider, so it cannot be a selection;
  showing it tells the person which model Claude Code sends.
- **Applies to:** `internal/adapter/claude` (the inspection returns the value), `adapter.Status`, the status row and
  the check's detail.

## Risks / Trade-offs

- A stale write of a running client is followed as a choice. The followed model and the returning command are
  printed, and the next Change words what a running client needs.
- The record in `aigw.toml` follows only when a command writes that client's files; a reader of the file itself, not
  of `aigw status`, sees the earlier model until then.
- The clients' writes are imitated in every test of this Change; the next Change's measurement with the real
  clients may change the conditions of following.

## Migration Plan

No data changes. A script that ran the Fix of a synchronization that found no added model receives the restoring
command from the next release on.
