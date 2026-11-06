## Context

`GatewayAbsent` in `internal/adapter/hermes/verification_home_unix.go` decides
whether a verification home's gateway is gone. The home's remover,
`releaseVerificationGateway` and the release journey's Hermes cleanup rely on
it. It checked two Hermes records: the `gateway.lock` flock and the process
named in `gateway_state.json`. The proposal names the window those records
leave open.

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
  `phase: exited` when it exits through its normal shutdown. A `running`
  record whose process is gone is Hermes's own definition of an unclean exit,
  so it also counts as ended.
- **An unconfirmed launch is not a running gateway.** A launch that ends
  before the gateway writes any lifecycle record, through an early guard exit,
  a failed spawn or a kill during boot, keeps its home. Once the bounds pass,
  the remover retains and names the home without claiming that a gateway still
  runs.
- **One reader per record.** `readGatewayLifecycle` parses the lifecycle record
  for both the ownership decision and the retention message, and both process
  checks use the same signal-0 test, `processExited`.

## Risks / Trade-offs

- A launch that fails before the gateway writes any lifecycle record keeps its
  home owned. Before this Change such a home was deleted. Cleanup now retains
  and names it after its bounds, which costs disk space and a verification
  error, but never deletes a home that a gateway may still re-create.
- These are Hermes's own file names in v0.22.7. If a later Hermes renames them,
  the adapter's tests against those names are the place that changes.
