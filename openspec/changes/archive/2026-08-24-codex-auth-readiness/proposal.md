## Why

AIGW currently reports an enabled Codex projection as Ready after it verifies
the projected configuration, executable, endpoint, and Token. That statement is
broader than the evidence: Codex native credential storage may still be
unbound. Operators need one truthful progression from setup through projection
and explicit authentication without making sync mutate native credentials.

## What Changes

- Separate projection readiness from native authentication readiness.
- Use the public, read-only Codex login-status command when it can prove the
  selected target is authenticated.
- Treat absent or unprovable native authentication as an explicit next action,
  never as successful readiness.
- Keep aigw sync projection-only and aigw adapter auth codex as the sole
  authentication mutation.
- Align human and JSON status output and document the resulting journey.

## Capabilities

### Modified Capabilities

- product-control-plane: reports bounded client readiness facts.
- progressive-team-onboarding: guides deferred Codex authentication after
  setup or later client installation.

## Impact

The change is confined to AIGW's Codex adapter inspection, readiness projection,
tests, and user documentation. It does not read Codex conversations, alter model
metadata, configure a proxy, or merge authentication into synchronization.
