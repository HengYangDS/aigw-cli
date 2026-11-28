## Context

`VERSION` and the first heading of `CHANGELOG.md` are the product's version and release chronology
(`docs/governance/change-and-release-policy.md`). The product Change on `dev` was archived in its own lane, so the
preparation needs a Change of its own to be written under.

## Goals / Non-Goals

**Goals:**

- One commit that names 0.12.0 in both files, on which the release proof, the candidate and the tag are made.

**Non-Goals:**

- Any product change. The entries under the new heading were written by the Change that made them.

## Decisions

### The release is 0.12.0

- **Rests on:** the pending entries are marked breaking: `aigw.toml` and team files move to version 9 and a version 8
  file is refused with its edits; `executable` becomes `program` or `extensions`; the client-native keys leave; a
  client's `forwarding` becomes one root; several JSON documents move to new schema versions. The series is below 1.0;
  0.5.0 to 0.11.0 each took the minor number for a breaking change.
- **Considered:** 0.11.1; 0.12.0.
- **Chosen because:** a member on 0.11.0 cannot read a version 9 file and every user edits `aigw.toml` once; the minor
  number is the signal. The owner named no number for this release; the acceptance owner of the repository chose it
  on 2026-11-28 and tells the owner.
- **Applies to:** `VERSION`, the changelog heading, the tag `v0.12.0`.

### The heading carries the UTC day of the preparation

- **Rests on:** the release policy's rule that an older heading needs its exact signed tag and date; the tag is made
  on or after the preparation's UTC day, and the proof refuses a heading dated after the current UTC day.
- **Considered:** the local day; the UTC day.
- **Chosen because:** every earlier heading is a UTC day.
- **Applies to:** `CHANGELOG.md`.

## Risks / Trade-offs

- The journeys that cross from the published 0.11.0 to this release (the version 8 refusal, the edits, the program
  rollback to the kept copy, the re-upgrade) run first in the candidate's acceptance against `v0.11.0`, since the
  release tool reads the candidate's version only from `VERSION`. A failure there needs a fix Change before the tag.
- Notarization needs the owner's console unlocked; the release waits for it after the candidate is accepted.
