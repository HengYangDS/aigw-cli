## Why

The 0.7.0 cutover on the owner's Mac on 2026-11-12 found four defects that the isolated rehearsal of task 8.6 could not
see, because they need the owner's real client files, a second team-file apply or a parent agent session:

- The first `aigw setup` after an upgrade refused Claude Code's settings, because the user had removed the `model` that
  0.6.1 projected. 0.7.0 has no earlier selection with which to recognize that as the user's model preference, and the
  fix it printed, `aigw sync`, cannot run without `aigw.toml`.
- A second `aigw setup --from` reset five model labels; its preview listed those models as changed without saying that their labels would change.
- `aigw check --depth client` failed Claude as an authentication rejection when run from a Claude Code session, because
  the client inherited that session's environment.
- Root help still describes AIGW as configuration "routing".

## What Changes

- The first setup after an upgrade accepts a Claude Code model preference when the model of the new selection
  reconstructs the recorded ownership digest, and keeps the user's preference, including its absence. Any other edit
  stays a conflict, and its refusal names a fix that runs in the current state.
- A team-file preview lists every change that its write makes, including model labels that the team file resets.
- Client verification starts each native client without the variables of a parent agent session.
- Root help describes AIGW without a retired word, and the retired-word test covers "routing".
- The repository's OpenSpec CLI moves to 2.0.0 under the owner's release-age exception of 2026-11-12, which covers that
  package alone; 2.0 changes no OpenSpec workflow, and the repository installs none of its agent skills.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `projection-format`: the first setup after an upgrade recognizes a model preference.
- `progressive-team-onboarding`: a team-file preview is complete.
- `client-adapters`: client verification runs in a clean environment.

## Impact

`internal/adapter/claude` (settings ownership), `internal/cli/setup` (team-file preview), the native client
verification of each adapter, root help, and `package.json` with `package-lock.json` for OpenSpec.
No configuration format changes. Release 0.7.1.

A family's display label stays settable only through a team file, as design.md of `command-surface-end-state` decided;
on 2026-11-12 a families-only team file restored the owner's seven labels.
