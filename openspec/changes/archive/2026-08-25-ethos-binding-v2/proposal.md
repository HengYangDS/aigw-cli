## Why

AIGW still carries the pre-schema repository Commitment. The current ETHOS
runtime can install hooks but cannot compile status, plan, or lane creation from
that carrier, forcing unsafe governance bypasses.

## What Changes

Migrate the existing Commitment to schema version 2 while preserving its
repository identity, scope, invariants, acceptance criteria, and authority
references. Add no compatibility carrier or parallel state.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This is a binding-schema migration only.

## Impact

The tracked repository Commitment becomes readable by the accepted ETHOS
runtime. Product behavior and AIGW runtime configuration do not change.
