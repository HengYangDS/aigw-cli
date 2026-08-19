## Context

See `proposal.md`. The existing Claude adapter already owns the official user
settings projection and credential-safe process plan.

## Goals / Non-Goals

**Goals:**

- Make the helper independent of shell PATH and valid for GUI launches.
- Keep one projection owner and preserve atomic rollback.

**Non-Goals:**

- No shell-profile mutation, token persistence in JSON, client interception, or
  Proxy lifecycle coupling.

## Decisions

1. Carry the executable path through the existing synchronizer rather than
   adding a second configuration channel.
2. Require an absolute path and quote it for the host shell grammar; reject
   control characters before any file write.
3. Keep the public helper command unchanged after the executable path, so the
   existing credential command remains the single credential owner.

## Risks / Trade-offs

- [A relocated executable invalidates the projection] -> the existing readiness
  and repair surfaces re-project the current installed path.
- [Shell quoting differs by platform] -> use the platform-native quoting branch
  and focused Windows/POSIX tests.

## Migration Plan

Existing marked Claude settings are re-projected on the next setup, repair, or
sync. Disable restores the captured user state. No user-owned file is migrated.
