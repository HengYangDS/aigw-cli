## Context

The installed acceptance of 0.7.0 on the owner's Mac on 2026-11-12 recreated the configuration with 0.7.0's commands,
and every field matched the 0.6.1 record. It needed two workarounds and found two more defects. The evidence is the
cutover record of task 9.2 of the archived Change `command-surface-end-state`.

## Goals / Non-Goals

**Goals:** the first setup after an upgrade works on the owner's real client files; a team-file preview shows every
change its write makes; a client-depth check gives the same answer inside and outside an agent session; help uses
the design's vocabulary.

**Non-Goals:** a command that sets a family label. The design of 0.7.0 keeps family labels in team files, and a
families-only team file restored the owner's seven labels.

## Decisions

### A first setup recognizes a model preference

Claude Code's sidecar records the hash of the managed values that an earlier release projected, `model` included.
Today the substitution that recognizes a user's model preference uses the previous selection's model. A first setup
has no previous selection, so a removed or changed `model` reads as an outside edit. On a first setup, the model of the
new selection takes the place of the previous one. The edit is accepted only when that substitution reproduces the
recorded hash exactly. Every other change stays a conflict.

Rejected: ignoring the sidecar on a first setup, which would overwrite real edits; and accepting any edit to `model`,
which would let an edit of the connection pass as a preference.

### A refusal without a configuration names a fix that runs

The fix "undo the edit, then run `aigw sync`" cannot run before `aigw.toml` exists. Without a configuration, the fix is
to undo the edit and run the refused `aigw setup` again. One owner renders this fix for every client adapter.

### A team-file preview is complete

The merge rule stays: a model that both the file and the configuration define takes the file's definition. The preview
and its JSON list each local model whose definition the file replaces, with the fields that change, labels included.

### Verification runs outside a parent agent session

Each adapter already removes the variables that it manages from a native client's environment. Beside that list it
declares the variables through which a parent agent session configures its own client, taken from each client's
official documentation, and removes them too. The user's own variables, such as `PATH`, `HOME`, proxies and locale,
stay.

### OpenSpec 2.0.0

The repository's OpenSpec CLI moves to 2.0.0, pinned exactly, under the owner's release-age exception of 2026-11-12
for that package alone. 2.0 changes no OpenSpec workflow. The repository installs none of its agent skills, so
`openspec update` changes nothing here.

## Risks / Trade-offs

- [The removed variable list drifts as clients add variables] → the list lives with each adapter and is tested with
  the variables set.
- [A recognized preference hides a deliberate edit of `model`] → only a model-only edit is accepted; the user's model
  stays as the user left it, as the existing preference scenarios require.
