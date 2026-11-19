## Context

`.config/checks/scc/policy.toml` holds a main specification to 500 non-blank lines, and while a Change is active its
delta stands in for the specification that it rewrites, so the gate measures the merged specification only after the
archive. `progressive-team-onboarding` stood at 495 after `setup-partial-tokens`; the archive of
`setup-token-outcomes` added 21 lines and gave 516. This lane rehearsed that archive on a copy and validated the merged
specifications strictly, and did not measure their size.

`repository-organization` states how requirements move: "Moving a requirement to another specification SHALL keep
every one of its scenarios, and a change that only moves requirements SHALL leave each of them unchanged, word for
word", with the scenario "Requirements move without a change". `specification-headroom` did so for four
specifications on 2026-11-16 and named `progressive-team-onboarding` among those that would need the same.

## Goals / Non-Goals

**Goals:**

- `progressive-team-onboarding` within its limit with room for the next Change, by a move that a comparison proves.

**Non-Goals:**

- Any change of a requirement's words, of the ceiling, or of another specification.

## Decisions

### The shipped team file becomes a capability of its own

- **Rests on:** the two subjects of the specification. Ten requirements state what `aigw setup`, `aigw sync`,
  `aigw config export` and `aigw provider add` do; two state what `manifests/team.toml` holds: which families it keeps
  for each vendor's line, and that it is reviewed, token-free and usable as it stands, with its recommendations and
  labels. The consumers of the specification's path, listed before the move: no tracked file outside
  `openspec/changes/archive` links to it; `command-line` and `secret-storage` name the capability for what setup
  does, which stays; `tools/ci` reads `openspec/specs` as a whole and names no capability.
- **Considered:** shorten the Token scenarios that the last Change wrote; raise the ceiling; move guided setup; move
  the shipped team file.
- **Chosen because:** text is never fitted to a count, and the ceiling is the size of one document unit and not this
  Change's to move. The shipped file's two requirements are the one group whose subject is a file of the repository
  and not a command, and every Change that curates the file (`team-file-lines`, `team-gpt-6-2`, `team-suheai`) edited
  them without touching setup. They hold 93 non-blank lines, which leaves the source at 423 and gives the new
  specification 99.
- **Applies to:** `openspec/specs/progressive-team-onboarding/spec.md` and the new
  `openspec/specs/shipped-team-file/spec.md`.

### The move is an ordinary commit, proved by comparing blocks

- **Rests on:** `specification-headroom`'s decision of the same name: a delta would state each moved requirement twice,
  as REMOVED and as ADDED in full, by hand.
- **Considered:** deltas for the two requirements; a direct move by a script.
- **Chosen because:** a direct move is one operation whose proof is a comparison: every requirement block of the
  source at the base stands byte for byte in exactly one of the two specifications at the head, and no other
  specification changes. The rule that the move follows is written already, so this Change needs no delta.
- **Applies to:** this Change.

## Risks / Trade-offs

- A reader who knew the shipped file's rules under `progressive-team-onboarding` finds them under `shipped-team-file`;
  archived Changes keep the old name, as history does.
- The lane's rehearsal of an archive now measures each merged specification as the size gate does, since strict
  validation alone let this through.
