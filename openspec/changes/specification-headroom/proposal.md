## Why

Four specifications stand within seven lines of the 500 that a specification may hold: `command-line` and
`product-quality` at 499, `secret-storage` at 495 and `cli-readiness` at 493. The next Change that adds one
scenario to any of them fails the size gate at its archive, after its review. On 2026-11-04 an archive grew
`product-quality` to 512 lines and failed the hosted quality job, and the last Change needed a simulated archive
to learn that `secret-storage` would hold.

## What Changes

- Move four cohesive groups of requirements, word for word, into capabilities of their own:
  `native-credential-store` out of `secret-storage`, `client-verification` out of `cli-readiness`,
  `command-output` out of `command-line` and `quantitative-quality` out of `product-quality`.
- Shorten the purpose of `cli-readiness`, `command-line` and `product-quality` by the clause that moved.
- No moved requirement and no scenario changes a word, and no behavior of the product changes.
- State how such a move is proved: every requirement of the sources stands byte for byte in exactly one
  specification afterwards.

## Capabilities

### New Capabilities

None by delta. The four capabilities above come into being by the move itself, in an ordinary commit.

### Modified Capabilities

- `repository-organization`: "Main specifications stay within one reading unit" gains the rule and the
  scenario of a move that changes nothing.

## Impact

- `openspec/specs/` gains four specifications and the four sources shrink to between 326 and 346 lines.
- No source file, test, command, document outside `openspec/` or release artifact changes.
- Out of scope: the specifications between 456 and 477 lines (`repository-organization`,
  `product-control-plane`, `model-selection`, `progressive-team-onboarding`), and any change of the ceiling.
