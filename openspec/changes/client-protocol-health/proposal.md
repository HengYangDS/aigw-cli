## Why

`aigw check` now evaluates every enabled client Route, but its shared health
probe still sends an OpenAI-style request for Claude. A healthy Claude Route can
therefore fail with an endpoint mismatch immediately after an otherwise valid
upgrade.

## What Changes

- Make the existing credential-validation boundary construct authentication
  probes from the selected client's declared protocol.
- Reuse that boundary from readiness diagnostics instead of maintaining a
  second request shape.
- Preserve bounded retry and failure classification while deleting the duplicate
  protocol-agnostic request construction.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The existing `product-control-plane` contract already requires every
enabled Route to pass its distinct authentication target; this change corrects
the implementation to satisfy it.

## Impact

The change is limited to credential validation, readiness diagnostics, their
tests, and the architecture dependency declaration. It adds no command, state,
dependency, provider branch, Proxy coupling, or compatibility path.
