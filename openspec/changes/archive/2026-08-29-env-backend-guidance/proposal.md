## Why

The environment credential backend is intentionally read-only, but several CLI
paths still tell operators to run `aigw rotate`. That command cannot modify an
environment variable, so the suggested recovery fails and interrupts setup or
client adoption instead of identifying the real owner of the missing Token.

## What Changes

- Report the exact `AIGW_TOKEN_<ACCOUNT>` variable whenever the selected secret
  backend is read-only.
- Reject `aigw rotate` before prompting or reading standard input when the
  backend cannot persist credentials.
- Keep `rotate` guidance only for writable credential backends.
- Reuse one invocation-level remediation policy across commands rather than
  duplicating backend-specific strings.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change aligns the implementation with the existing progressive team
onboarding contract for independently supplied Tokens and executable next
steps.

## Impact

Only CLI remediation text, early validation, and their regression tests change.
Configuration, secret ownership, provider behavior, and client projection
formats remain unchanged.
