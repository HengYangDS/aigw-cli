## Context

`.config/checks/scc/policy.toml` allows a specification 500 non-blank lines. While a Change is active the size
gate measures its delta, so a specification that a Change grows past the ceiling fails only at the archive
commit. Read at `dbcded17`: `command-line` 499, `product-quality` 499, `secret-storage` 495, `cli-readiness` 493.

## Goals / Non-Goals

**Goals:**

- Each of the four specifications has room for its next Change.
- A reader can prove that the move changed no requirement.

**Non-Goals:**

- A new ceiling, or a new rule for when a specification is split.
- Any change of a requirement's or a scenario's words.

## Decisions

### Four groups leave the specifications that are full

- **Rests on:** the four sizes above; the failed archive of 2026-11-04 (`product-quality` at 512); and the
  consumers of a specification's path, listed before the move at `dbcded17`: no tracked file outside
  `openspec/changes/archive` links to one of the four files, the one active Change holds deltas for
  `ci-diagnostics` and `product-control-plane` alone, and `tools/ci` reads `openspec/specs` as a whole and
  names no capability.
- **Considered:** raise the ceiling; split a specification inside the Change that next needs room in it; move
  cohesive groups now, in a Change that does nothing else.
- **Chosen because:** the ceiling is the size of one document unit and is not this Change's to move. A split
  inside a Change of behavior mixes moved sentences with changed ones, and a reviewer cannot tell them apart.
  A move alone is proved by comparing blocks: every requirement block of the four sources at the base stands
  byte for byte in exactly one specification at the head.
- **Applies to:** the four specifications and the four new ones. The other specifications keep their place;
  four of them hold between 456 and 477 lines and will need the same when a Change grows them.

| From              | To                        | Requirements                                                                                       |
| ----------------- | ------------------------- | -------------------------------------------------------------------------------------------------- |
| `secret-storage`  | `native-credential-store` | the six whose subject is the operating system's credential service, the listing by name among them |
| `cli-readiness`   | `client-verification`     | the four of a Hermes verification, the one of a real Codex session and the one on warning symbols  |
| `command-line`    | `command-output`          | the seven on who owns output, the layout and naming of human output and the JSON contract          |
| `product-quality` | `quantitative-quality`    | the four on quantitative evidence, the coverage authority, derived policy and coverage attempts    |

### The move is an ordinary commit, and the one delta states its proof

- **Rests on:** a delta states a moved requirement twice, as REMOVED with a reason and as ADDED in full, for
  the 23 requirements here. A Change without any delta must declare `skip_specs: true`; OpenSpec 1.16.0 then
  reports one INFO finding for it, and this repository's OpenSpec gate fails on every finding at every
  severity, as `product-quality` requires (both measured in this lane on 2026-11-16).
- **Considered:** deltas for every moved block; a Change without deltas and a gate that admits its
  declaration; a direct move with one delta for the rule that the move follows.
- **Chosen because:** 23 blocks restated by hand are 23 places where a word can change unseen, and a direct
  move is one operation by a script whose proof is the comparison of blocks. A gate that admits one kind of
  finding is a change of the quality policy and not this Change's. The rule of a move without a change was
  unwritten; `repository-organization` now states it, and that is the Change's one delta.
- **Applies to:** this Change, and every later one that only moves requirements. A Change of behavior states
  its deltas as before.

## Risks / Trade-offs

- A reader who follows an older link to a requirement by its specification's name finds it in another file →
  the requirement's title is unchanged, and a search by title finds it.
- A Change that is drafted elsewhere against one of the four sources would name a requirement that has moved →
  none exists: the active Change touches other specifications.

## Migration Plan

One commit moves the blocks. Reverting that commit restores the four specifications.
