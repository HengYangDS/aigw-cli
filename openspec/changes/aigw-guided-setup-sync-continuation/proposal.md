## Why

Guided setup currently directs users to verification even when no supported
client is installed, although only synchronization can discover and project a
client installed later. This contradicts the existing progressive onboarding
contract and leaves a successfully connected Account without an actionable
next step.

## What Changes

- Make guided setup choose its continuation from the resulting client state.
- Direct users with no configured client to install one and run `aigw sync`.
- Preserve `aigw check` as the continuation when setup configured a client.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: Make the guided setup continuation explicit
  when no supported client is installed.

## Impact

The change is limited to guided setup presentation and its acceptance test. It
does not change configuration, credential, discovery, or projection ownership.
