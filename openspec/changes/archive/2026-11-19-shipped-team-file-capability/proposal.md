## Why

The archive of `setup-token-outcomes` grew the specification `progressive-team-onboarding` from 495 to 516 non-blank
lines, above the 500 that a main specification may hold, and the proof of the 0.8.3 release head stopped at the
source-size gate (2026-11-19, 21:41 CST). The specification holds two subjects: what `aigw setup` does with a team
file, and what the team file that this repository ships contains.

## What Changes

- Move the two requirements on the shipped team file, word for word, from `progressive-team-onboarding` into a
  capability of their own, `shipped-team-file`: "The shipped team file keeps one general family per other vendor" and
  "The shipped team file is reviewed and directly usable".
- State the purpose of `shipped-team-file`. The purpose of `progressive-team-onboarding` names setup alone already and
  stays as it is.
- No moved requirement and no scenario changes a word, and no behavior of the product changes.

Out of scope: any change of the ceiling, and the other specifications near it (`repository-organization` at 484,
`product-control-plane` at 466, `model-selection` at 464), which move when a Change grows them.

## Capabilities

### New Capabilities

None by delta. `shipped-team-file` comes into being by the move itself, in an ordinary commit, as
`repository-organization` lets a Change that only moves requirements do.

### Modified Capabilities

None. This Change changes no requirement, so it declares `skip_specs: true`.

## Impact

`openspec/specs/progressive-team-onboarding/spec.md` shrinks and `openspec/specs/shipped-team-file/spec.md` is new. No
source file, test, command, document outside `openspec/` or release artifact changes.
