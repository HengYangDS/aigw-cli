## Why

When `aigw verify --for hermes` times out, the Hermes gateway that the
verification chat launched can still be starting. It has written its launch
log, but it has not yet taken its lock or recorded its process. AIGW's cleanup
reads only the lock and the process record, so it sees no gateway and deletes
the private home. The gateway then re-creates the home a moment later.

The re-created home holds only Hermes's own settings, without AIGW's two-second
idle exit. The gateway therefore runs for Hermes's default ten minutes, in a
home that verification reported as reclaimed. On 2026-11-05 a timed-out run
left such a home and gateway. The 2026-10-27 model refresh saw 34 of 35 homes
come back the same way.

## What Changes

- A verification home whose gateway launch log exists counts as owned by a
  gateway until Hermes's lifecycle record says the gateway exited, or names a
  process that is gone.
- Cleanup then waits within its existing bounds instead of deleting a home
  that a starting gateway will re-create. If the bounds pass, it retains and
  names the home, as it already does for a running gateway.

Non-goals: the verification turn bound, the gateway's idle exit and the
operator's own Hermes home do not change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `cli-readiness`: Hermes verification deletes its home only after a gateway
  launched into it has ended, and does not claim a gateway runs without
  evidence.

## Impact

`internal/adapter/hermes` changes, and the next release ships the fix.
Configuration, credentials and the native Hermes installation do not change.
