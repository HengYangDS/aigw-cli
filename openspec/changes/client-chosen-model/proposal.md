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
- Claude Code keeps a model of its own, and `aigw status` names only AIGW's model for it.

## What Changes

- Every command works from the configuration that follows a model chosen in Codex or Hermes. A command that saves
  the configuration records the choice together with AIGW's record of the client's file, keeps the client's value
  and keeps its own change; `aigw status` and `aigw check` report the chosen model as the selection;
  `aigw client disable` and `aigw self uninstall` withdraw after a choice.
- A command that records a choice, its preview, `aigw status`, `aigw check` and bare `aigw` name each followed
  selection, the earlier one and the command that returns to it: a section ahead of the next step in text,
  `followed_selections` in JSON.
- **Breaking:** `aigw sync --json` names a followed selection in `followed_selections`; `adopted_selections` is
  gone.
- **Breaking:** when a client's file names a model that AIGW has not added, the Fix is the command that restores the
  recorded selection, and the failure's text names the command that adds the named model. The Fix no longer follows
  the file's value.
- `aigw config rollback` restores each client's selection as the restored configuration holds it, a model chosen in
  the client since then included, and names each one with the model that it replaces (`selections`), in its preview
  and its result.
- A change that is refused before anything was written reads as the refusal behind it and says that configuration
  and client files are unchanged, from a command and from its preview alike; `aigw config rollback` passes such a
  refusal on with its Fix, where it reported a rollback that did not complete.
- `aigw use hermes` replaces a Hermes selection that names a model AIGW has not added, as `aigw use codex` does. An
  edited address or mode of Hermes's selection stays an outside edit, for `aigw use hermes` too.
- `aigw use` reports whether the stored configuration changes (`changed`) alike in its preview and its result.
- `aigw status` and `aigw check` name the model that Claude Code's own settings select (`client_model`).
- The documents of the saving commands, of `aigw status` and of `aigw check` move up one `schema_version` each.

Authority: the new capability `client-chosen-model` owns what a model chosen in a client means to every command;
`projection-format` keeps the byte-level ownership of client files; the clients keep their sessions and pickers.
Reuse: the adoption that `aigw sync` already performs, moved to the one place where a command reads the
configuration, and the transaction that already writes client files with AIGW's records of them.

Out of scope: what a real Codex or Hermes writes when a model is chosen in it, and what a running client does after
`aigw use`, which the next Change in this lane measures with the real clients before any sentence about them is
written; following a model for Claude Code or Claude Desktop, whose own choices AIGW does not record; a change to
any other managed setting, which still refuses; and a rollback of a setting, such as a retry budget, that does not
reach the client's file, which this Change found and leaves to its own Change.

## Capabilities

### New Capabilities

- `client-chosen-model`: a model chosen in a client is its selection for every command and is recorded with AIGW's
  record of the client's file; commands and reports name what they follow; a model that AIGW has not added is
  reported with the command that restores and the command that adds; an explicit selection wins; a rollback names
  the selections that it restores; Claude Code's own model is shown.

### Modified Capabilities

- `model-selection`: "A client uses one model" leaves a model chosen in another client to `client-chosen-model`; the
  requirement that only synchronization adopts a client's choice is removed.
- `projection-format`: a Codex root model that changed is followed by every command that writes Codex's files, and
  the one recovery action of an outside edit does not apply to a client's own model selection.

## Impact

`internal/app` (the one load of the configuration), `internal/app/synchronization` (the working configuration, the
transaction's preimage, the record of what an invocation followed and the planning step that a command and its
preview share), `internal/store` (the preimage against a view), `internal/adapter` with its Codex, Hermes and Claude
Code packages, `internal/platform` (the words that a shell passes through, moved from the command layer),
`internal/terminal` (the invocation's fields of a document, the section ahead of the next step and the presentation
of a refusal before a write), `internal/cli` and its commands, `cmd/aigw/testdata/script`,
`docs/concepts/product-concepts.md` and `CHANGELOG.md`.
