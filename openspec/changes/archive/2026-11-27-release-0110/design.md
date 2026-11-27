## Context

`VERSION` and the first heading of `CHANGELOG.md` are the product's version and release chronology
(`docs/governance/change-and-release-policy.md`). The product Changes on `dev` were archived in their own lanes, so
the preparation needs a Change of its own to be written under.

## Goals / Non-Goals

**Goals:**

- One commit that names 0.11.0 in both files, on which the release proof, the candidate and the tag are made.

**Non-Goals:**

- Any product change. The entries under the new heading were written by the Changes that made them.
- The schema 9 Change that the owner ordered after the missing-program Change on 2026-11-25; it is not on `dev`.

## Decisions

### The release is 0.11.0

- **Rests on:** four pending entries are marked breaking: a model that declares `context_window` needs this release
  or later, which 0.10.0 refuses; AIGW never replaces the executable that `aigw.toml` names, and a client whose
  executable is gone stops; AIGW's record of Codex's model table names its producer, and `aigw check --depth local`
  fails on a difference; and only macOS on ARM64 and Linux on ARM64 and AMD64 are released. The series is below 1.0;
  0.5.0 to 0.10.0 each took the minor number for a breaking change.
- **Considered:** 0.10.1; 0.11.0.
- **Chosen because:** a member on 0.10.0 cannot apply the shipped team file, a command that passed now stops, and the
  Windows and Intel macOS archives are gone; the minor number is the signal. The owner named no number for this
  release; the acceptance owner of the repository chose it on 2026-11-27 and tells the owner.
- **Applies to:** `VERSION`, the changelog heading, the tag `v0.11.0`.

### The heading carries the UTC day of the preparation

- **Rests on:** the release policy's rule that an older heading needs its exact signed tag and date; the tag is made
  on or after the preparation's UTC day, and the proof refuses a heading dated after the current UTC day.
- **Considered:** the local day; the UTC day.
- **Chosen because:** every earlier heading is a UTC day.
- **Applies to:** `CHANGELOG.md`.

## Risks / Trade-offs

- A program rollback from 0.11.0 to 0.10.0 is exercised by the candidate's acceptance against the published
  `v0.10.0`; 0.10.0 refuses a configuration that declares windows, so what the rollback refuses is recorded in the
  release record, not assumed here.
- Notarization needs the owner's console unlocked; the release waits for it after the candidate is accepted.
