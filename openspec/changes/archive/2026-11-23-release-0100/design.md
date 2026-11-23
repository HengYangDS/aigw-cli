## Context

`VERSION` and the first heading of `CHANGELOG.md` are the product's version and release chronology
(`docs/governance/change-and-release-policy.md`). The product Changes on `dev` were archived in their own lanes, so
the preparation needs a Change of its own to be written under.

## Goals / Non-Goals

**Goals:**

- One commit that names 0.10.0 in both files, on which the release proof, the candidate and the tag are made.

**Non-Goals:**

- Any product change. The entries under the new heading were written by the Changes that made them.
- The declared windows of Codex and Hermes (lane `20261121-client-windows`), which wait for the owner's answer on
  their rollback and are not on `dev`.

## Decisions

### The release is 0.10.0

- **Rests on:** five pending entries are marked breaking: a model chosen in Codex or Hermes is the client's selection
  for every command, with `followed_selections` in place of `adopted_selections`; the Fix for a file that names a
  model AIGW has not added restores the recorded selection; a command that names a client without a Token checks its
  files and refuses where it passed over them; `aigw setup --from` removes the local-only models and families that
  the team file does not describe; and its `--prune` flag is removed. The series is below 1.0; 0.5.0 to 0.9.0 each
  took the minor number for a breaking change.
- **Considered:** 0.9.1; 0.10.0.
- **Chosen because:** a script that reads `adopted_selections` or passes `--prune` breaks, a command that passed now
  refuses, and an import removes models it kept; the minor number is the signal. The owner named no number for this
  release; the acceptance owner of the repository chose it on 2026-11-24 and tells the owner.
- **Applies to:** `VERSION`, the changelog heading, the tag `v0.10.0`.

### The heading carries the UTC day of the preparation

- **Rests on:** the release policy's rule that an older heading needs its exact signed tag and date; the tag is made
  on or after the preparation's UTC day, and the proof refuses a heading dated after the current UTC day.
- **Considered:** the local day; the UTC day.
- **Chosen because:** every earlier heading is a UTC day.
- **Applies to:** `CHANGELOG.md`.

## Risks / Trade-offs

- A program rollback from 0.10.0 to 0.9.0 is exercised by the candidate's acceptance against the published `v0.9.0`;
  what it refuses, if anything, is recorded in the release record, not assumed here.
- Notarization needs the owner's console unlocked; the release waits for it after the candidate is accepted.
