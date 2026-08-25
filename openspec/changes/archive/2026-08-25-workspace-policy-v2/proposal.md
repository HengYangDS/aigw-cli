## Why

The repository still carries the retired transition-row workspace shape. The
current ETHOS runtime can read ordinary status but cannot authorize accepted
closeout from that incomplete policy, leaving protected-ref lifecycle blocked.

## What Changes

Replace the legacy transition row with the complete current branch-role table.
Keep `main`, `dev`, `candidate/dev`, `work/*`, and `proposal/*` as the same
semantic roles while declaring accepted-to-release fast-forward mirroring and
canonical sibling worktrees explicitly.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This is a governance-carrier migration, not a product behavior change.

## Impact

ETHOS can authorize lane creation, prewrite, candidate landing, accepted
closeout, and exact peer publication without a compatibility fallback.
