## Context

`client-chosen-model` landed in `dev` at `06edce9a4` on 2026-11-23. Its twelfth review (2026-11-22, brief
`build/runtime/work/review-12-chosen/brief.md` in the canonical checkout, probes `zz12_review_*_test.go` beside it)
named two facts that each have two sources. This Change removes the first: "which of a client's files does a
transition drop". The review's record is `build/evidence/selection-states-20261121/README.md`, section "Version twelve
and the twelfth review".

## Goals / Non-Goals

**Goals:** one owner of the judgement "do these two configured paths name one file"; every decision about kept and
dropped client files made by it; the review's findings 1 and 4 failing first and passing after.

**Non-Goals:** the second root cause (whether a Token is usable when its store fails), which is its own Change;
changing what AIGW writes into a file; resolving paths that do not exist yet beyond their cleaned spelling.

## Decisions

### The judgement lives in `internal/platform`

- **Rests on:** `aigw client edit` (`internal/cli/client`), synchronization (`internal/app/synchronization`) and both
  adapters already import `internal/platform`, and the judgement needs the file system, which `internal/domain` may
  not use (the architecture gate).
- **Considered:** an exported function in `internal/adapter`; a method on `domain.ClientBinding` that takes a stat
  function.
- **Chosen because:** one package that every caller already depends on, with no new dependency direction; the
  adapters keep their own reasons for resolving paths (Codex's canonical path, its state file) and ask only this
  question of it.
- **Applies to:** `sameTargetFile` and its `statTarget` hook move from `internal/adapter/codex/targets.go`; a function
  that returns the paths of one list that name no file of another replaces the four string comparisons.

### A path that does not exist compares by its cleaned spelling

- **Rests on:** Codex's rule today, and a target that a command creates has no identity to compare before it exists.
- **Chosen because:** it keeps the outcome that `client-chosen-model` accepted for new files, and the review's
  findings concern files that exist.

## Risks / Trade-offs

- **A stat per comparison:** client target lists hold one to a few files; the cost is negligible.
- **A file replaced between two stats** (another program renames over it) is compared by the identity it has when
  read; each command reads its targets once, as today.
