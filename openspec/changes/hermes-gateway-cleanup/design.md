## Context

`GatewayAbsent` in `internal/adapter/hermes/verification_home_unix.go` decides
whether a verification home's gateway is gone, and both the home's remover and
`releaseVerificationGateway` rely on it. It checks two Hermes records: the
`gateway.lock` flock and the process named in `gateway_state.json`. The
proposal names the window those records leave open.

## Goals / Non-Goals

**Goals:**

- A gateway that is still starting keeps its home owned.
- A gateway that ended, cleanly or not, releases it.

**Non-Goals:**

- Stopping a starting gateway early, or changing how long verification waits.

## Decisions

- **Launch log as the start signal.** Hermes's `spawn_unmanaged_gateway` writes
  `<home>/logs/gateway-stdio.log` before the gateway runs. On macOS the
  launchd job's shell appends to it; elsewhere the spawner opens it. A home
  with that log has had a gateway launched into it.
- **Lifecycle record as the end signal.** The gateway writes
  `<home>/state/gateway.lifecycle.json` with `phase: running` at boot and
  `phase: exited` on every clean exit. A `running` record whose process is
  gone is Hermes's own definition of an unclean exit, so it also counts as
  ended.
- **One liveness test.** Both the process record and the lifecycle record use
  the same signal-0 check, `processExited`.

## Risks / Trade-offs

- A launch that fails before the gateway writes any lifecycle record keeps its
  home owned. Cleanup then retains and names the home after its bounds, which
  costs disk space but never deletes a home that a gateway still uses.
- These are Hermes's own file names in v0.22.7. If a later Hermes renames them,
  the adapter's tests against those names are the place that changes.
