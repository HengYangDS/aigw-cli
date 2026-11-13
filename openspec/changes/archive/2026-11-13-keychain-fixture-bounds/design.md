## Context

`internal/secret/native` runs real Keychain operations on private test keychains. The tests call
`/usr/bin/security` themselves to create, observe and delete fixture items, and run child test processes that
perform AIGW's own native operations. Each of those test-side calls had a five-second bound, the same value as the
product's `operationTimeout`.

## Goals / Non-Goals

**Goals:**

- A test's own fixture or observation call fails only on a hang, not on a slow shared CI host.
- One name owns that bound.

**Non-Goals:**

- Changing AIGW's own native deadlines. The second failure of 2026-11-13 came from the product's one-second
  `securityToolTimeout` in the macOS migration; whether that bound should change is the review of 2026-11-20, with
  measurements.

## Decisions

### One fixture bound of 30 seconds

`keychainFixtureTimeout` replaces the six test-side five-second bounds: the find, dump and fixture commands, and
the wait for a child test process. 30 seconds leaves room for a host far slower than a quiet one, which the
`product-quality` requirement on hang bounds asks for, while a regression that blocks on a UI prompt still fails.
The child keeps its own five-second product deadline inside, so the parent's bound no longer races it.

Rejected: removing the test bounds, which would let a prompt block the suite until the package timeout; and
retrying a failed fixture call, which would hide a real slowdown.

## Risks / Trade-offs

- A test that hangs now takes up to 30 seconds to fail instead of five.
