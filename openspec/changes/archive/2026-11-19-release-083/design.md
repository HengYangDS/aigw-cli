## Context

`VERSION` and the first heading of `CHANGELOG.md` are the product's version and release chronology
(`docs/governance/change-and-release-policy.md`). The lane's two product Changes were archived before the release
was prepared, so the preparation needs a Change of its own to be written under.

## Goals / Non-Goals

**Goals:**

- One commit that names 0.8.3 in both files, on which the release proof, the candidate and the tag are made.

**Non-Goals:**

- Any product change. The entries under the new heading were written by the Changes that made them.

## Decisions

### The release is 0.8.3

- **Rests on:** the owner named it 0.8.3 on 2026-11-19. The series is below 1.0, and 0.8.2 carried a breaking entry
  in a patch release as well (the refusal of two models of one provider with one label).
- **Considered:** 0.9.0 for the breaking entries (the team setup's new flow and its JSON schema version 4); 0.8.3.
- **Chosen because:** the breaking entries remove a refusal and add fields; a script that parsed the old document
  reads `schema_version` and can tell. The owner's name for the release stands.
- **Applies to:** `VERSION`, the changelog heading, the tag `v0.8.3`.

### The heading carries the UTC day of the preparation

- **Rests on:** the release policy's rule that an older heading needs its exact signed tag and date; the tag is made
  on or after the preparation's UTC day.
- **Considered:** the local day; the UTC day.
- **Chosen because:** every earlier heading is a UTC day.
- **Applies to:** `CHANGELOG.md`.

## Risks / Trade-offs

- A script that relied on a rejected piped Token leaving the configuration untouched meets the new behavior in a
  patch release; the changelog marks both entries as breaking, and the command's exit status is unchanged.
