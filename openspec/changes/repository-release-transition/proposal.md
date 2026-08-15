## Why

The repository already verifies accepted source but does not declare the governed transition from `dev` to `main`. The missing positive policy forces release promotion to stop after proof.

## What Changes

- Declare one proof-bound `accepted-to-release` transition.
- Make repository governance require that transition.

## Impact

Verified accepted source can advance to the release branch through the public ETHOS command without raw Git mutation.
