## Why

A token-free team catalogue currently tells environment-backed users to set every
listed Account Token and then run `aigw check`. That contradicts progressive
onboarding: one compatible Account is sufficient, and `check` cannot activate a
newly connected Account or a client installed later.

## What Changes

- Make a catalogue-only import recommend one account-scoped connection action.
- Make environment-backed guidance name one Account variable at a time without
  implying that every provider Token is required.
- Direct users to `aigw sync` after credentials or clients become available;
  reserve `aigw check` for verification after activation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: require one truthful, account-scoped next action
  for deferred activation.

## Impact

This changes only setup result guidance, its acceptance tests, and the existing
progressive-onboarding specification. It adds no command, configuration field,
dependency, compatibility path, or runtime authority.
