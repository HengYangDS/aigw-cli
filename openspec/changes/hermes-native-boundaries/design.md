# Design

## Context

`hermesconfig.GatewayAbsent` reports a verification home free of its gateway
when no process holds `<home>/gateway.lock`. `AwaitGatewayAbsence`, the single
native stop in `releaseVerificationGateway`, and the remover returned by
`CreateVerificationHome` all decide with it.

Hermes 0.22.5 ends a graceful gateway run in
`gateway/run.py::_exit_after_graceful_shutdown`, which calls `_release_locks`
(pid file and runtime lock), then `_mark_exited` (exit metrics and lifecycle
ledger), then `_drain_logs` (up to one second), then `os._exit`. The gateway
writes `gateway_state.json`, its runtime status with `pid` and `start_time`,
under its home and keeps it after it stops (`retained_gateway_state`).

`aigw use` renders its selection with `presentation.Rows`. The client registry
`configuration.ClientSpec` already carries client facts that shape this output,
such as `RestartAfterProjection` for Claude Desktop.

## Goals / Non-Goals

**Goals:** a verification home is deleted only after its recorded gateway
process exits; the existing bounds and retention error stay as they are;
Hermes users learn from `aigw use` that existing sessions keep their route.

**Non-Goals:** signals or forced termination; changes to Hermes, its sessions or
the operator's Hermes home; a new waiting mechanism; JSON output changes;
notes for clients whose session behavior has not been verified.

## Decisions

### The gateway check also requires the recorded process to have exited

`GatewayAbsent` keeps the lock test and then reads `pid` from
`<home>/gateway_state.json`. The home is free only when that process no longer
exists, tested with `kill(pid, 0)`; signal 0 delivers nothing, it only reports
whether the process exists. A missing state file means no gateway recorded a
run, so the lock alone decides, as before. An unreadable or invalid file is an
error, so the remover keeps the home. Capturing the process identifier while
the gateway runs was rejected: the pid file is removed first on exit, while the
retained state file needs no timing. Comparing `start_time` against process
reuse was rejected because the bound already ends any wait, and a reused
identifier only keeps the home in place.

### The registry states which clients keep a session's route

`ClientSpec.SessionsKeepRoute` is true for Hermes only, the one client whose
session route retention this host verified. `aigw use` adds the "Existing
sessions" row when the projection changed, and its preview adds it when a
planned target changes state. A Hermes-specific condition in the renderer was
rejected because the registry already owns such client facts.

## Risks / Trade-offs

- [A gateway whose process does not exit within the bounds] → verification
  keeps and names the home instead of deleting it, as for a held lock.
- [A recycled process identifier] → the home is kept until the bound and then
  named; deletion never happens under a live process.

## Migration Plan

None.

## Open Questions

- None.
