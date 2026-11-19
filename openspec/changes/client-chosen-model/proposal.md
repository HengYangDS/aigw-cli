## Why

The owner's rule of 2026-11-20: a model chosen inside a client counts, and AIGW's record follows it; only a model that
AIGW has not added is not followed, and AIGW changes a client's model only on an explicit `aigw use`. Release 0.8.3
follows a client's choice in `aigw sync` alone. A run of 0.8.3 in an isolated home on 2026-11-20, with the clients'
writes imitated, showed what a person meets after choosing a model in Codex or Hermes:

- `aigw status` reports the client as invalid ("Codex configuration drift", "Hermes configuration cannot be
  inspected").
- A command that writes the client's files, such as `aigw provider edit`, refuses with "Undo the outside edit".
- When the client's file names a model that AIGW has not added, the Fix of `aigw sync` for Codex follows that value,
  and for Hermes both `aigw sync` and `aigw use hermes` refuse, so no command restores the selection.
- Claude Code keeps a model of its own, and `aigw status` still names AIGW's model for it.

## What Changes

- Every command that writes or previews a client's files first follows a model that the person chose in Codex or
  Hermes: it records the model as the client's selection, keeps the client's value, and names the followed model and
  the command that returns to the earlier one.
- `aigw status` and `aigw check` report a chosen model as the client's selection, marked as chosen in the client.
- **Breaking:** when a client's file names a model that AIGW has not added, the Fix is the command that restores the
  recorded selection, and the result also names the command that adds the named model. The Fix no longer follows
  the file's value.
- `aigw use hermes` replaces a Hermes selection that names a model AIGW has not added, as `aigw use codex` does.
- `aigw status` and `aigw check` name the model that Claude Code's own settings select.

Authority: the new capability `client-chosen-model` owns what a model chosen in a client means to every command;
`projection-format` keeps the byte-level ownership of client files; the clients keep their sessions and pickers.
Reuse: the adoption that `aigw sync` already performs, moved to the one place where a transition is planned.

Out of scope: what a real Codex or Hermes writes when a model is chosen in it, and what a running client does after
`aigw use`, which the next Change in this lane measures with the real clients before any sentence about them is
written; following a model for Claude Code or Claude Desktop, whose own choices AIGW does not record; and a change to
any other managed setting, which still refuses.

## Capabilities

### New Capabilities

- `client-chosen-model`: a model chosen in a client is its selection for every command; a model that AIGW has not
  added is reported with the command that restores and the command that adds; an explicit selection wins.

### Modified Capabilities

- `model-selection`: the requirement that only synchronization adopts a client's choice is removed; its scenarios
  move to `client-chosen-model`.
- `projection-format`: a Codex root model that changed is followed by every command that writes Codex's files, and
  the one recovery action of an outside edit does not apply to a client's own model selection.

## Impact

`internal/app/synchronization` (the transaction and the preview), `internal/adapter` with its Codex, Hermes and
Claude Code packages, `internal/app` (status), `internal/cli/invocation`, `internal/cli/status`, `internal/cli/check`
and `internal/cli/sync`, `cmd/aigw/testdata/script`, `README.md`, `docs/` where they describe adoption, and
`CHANGELOG.md`. The status and check documents gain fields; a client-file row gains the followed model.
