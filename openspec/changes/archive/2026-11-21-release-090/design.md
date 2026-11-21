## Context

`VERSION` and the first heading of `CHANGELOG.md` are the product's version and release chronology
(`docs/governance/change-and-release-policy.md`). The product Changes on `dev` were archived in their own lanes, so
the preparation needs a Change of its own to be written under.

## Goals / Non-Goals

**Goals:**

- One commit that names 0.9.0 in both files, on which the release proof, the candidate and the tag are made.

**Non-Goals:**

- Any product change. The entries under the new heading were written by the Changes that made them.
- The 1M window for Codex and Hermes, which is its own Change after this release.

## Decisions

### The release is 0.9.0

- **Rests on:** the pending entry of `client-model-windows` is marked breaking: AIGW's record of Claude Code's
  settings gains a field that 0.8.3 refuses, so the program cannot be rolled back to 0.8.3 over a projected Claude
  Code without `aigw client disable claude` first. The series is below 1.0; 0.5.0 to 0.8.0 each took the minor
  number for a breaking change, and 0.8.2 and 0.8.3 carried a smaller one in a patch number, 0.8.3 by the owner's
  own name for it.
- **Considered:** 0.8.4; 0.9.0.
- **Chosen because:** the earlier patch releases' breaking entries removed a refusal or added fields that a reader
  could tell by `schema_version`. This one changes a file of record that the earlier program refuses, which a user
  meets only when rolling back; the minor number is the signal that a rollback of the program is not free. The owner
  named no number for this release; the acceptance owner of the repository chose it on 2026-11-21 and told the
  owner.
- **Applies to:** `VERSION`, the changelog heading, the tag `v0.9.0`.

### The heading carries the UTC day of the preparation

- **Rests on:** the release policy's rule that an older heading needs its exact signed tag and date; the tag is made
  on or after the preparation's UTC day.
- **Considered:** the local day; the UTC day.
- **Chosen because:** every earlier heading is a UTC day.
- **Applies to:** `CHANGELOG.md`.

## Risks / Trade-offs

- A Claude Desktop session started from the app with the new profile has not been observed; the measurement ran
  Desktop's own Claude Code by hand with Desktop's variable set (`client-model-windows`, task 3.2). The installed
  release is the first place where the app's own session can be read, and the release record says what it shows.
- A user who had added `modelOverrides` by hand to settings that AIGW already owned meets a refusal after the
  upgrade until the key is removed; the changelog entry says so.
