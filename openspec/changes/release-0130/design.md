## Context

`VERSION` and the first heading of `CHANGELOG.md` are the product's version and release chronology
(`docs/governance/change-and-release-policy.md`). The product Changes on `dev` were archived in their own lanes, so
the preparation needs a Change of its own to be written under.

## Goals / Non-Goals

**Goals:**

- One commit that names 0.13.0 in both files, on which the release proof, the candidate and the tag are made.

**Non-Goals:**

- Any product change. The entries under the new heading were written by the Changes that made them.

## Decisions

### The release is 0.13.0

- **Rests on:** one pending entry is marked breaking: Codex's picker no longer lists Codex's bundled models or other
  providers' models, a bundled model leaves it until `aigw model add` adds it, and a provider with no model Codex can
  describe leaves it empty. The series is below 1.0; 0.5.0 to 0.12.0 each took the minor number for a breaking change.
  No configuration file changes version, so installing needs no edit of `aigw.toml`.
- **Considered:** 0.12.1; 0.13.0.
- **Chosen because:** what a user sees in Codex changes on upgrade; the minor number is the signal. The owner named no
  number for this release; the acceptance owner of the repository chose it on 2026-11-28 and tells the owner.
- **Applies to:** `VERSION`, the changelog heading, the tag `v0.13.0`.

### The heading carries the UTC day of the preparation

- **Rests on:** the release policy's rule that an older heading needs its exact signed tag and date; the tag is made
  on or after the preparation's UTC day, and the proof refuses a heading dated after the current UTC day.
- **Considered:** the local day; the UTC day.
- **Chosen because:** every earlier heading is a UTC day.
- **Applies to:** `CHANGELOG.md`.

## Risks / Trade-offs

- The journeys that cross from the published 0.12.0 to this release run first in the candidate's acceptance against
  `v0.12.0`, since the release tool reads the candidate's version only from `VERSION`. A failure there needs a fix
  Change before the tag.
- Notarization needs the owner's console unlocked; the release waits for it after the candidate is accepted.
- How Codex Desktop renders a picker with no listed entry was not observed; Codex's app-server, which Desktop reads,
  returns an empty list, and Codex's terminal picker then shows one line and keeps the selected model.
