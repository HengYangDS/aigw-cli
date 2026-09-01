## Why

AIGW already models Codex Responses Proxy as an optional Account endpoint, but
one deployment-specific requirement still implies that selected Codex routes
must traverse the Proxy. `aigw verify` also sends its own HTTP request for
Codex, so a green result does not prove that the configured Codex executable can
use the AIGW-owned projection and authentication state.

## What Changes

- Make direct HTTPS endpoints and independently operated gateways equivalent
  Account endpoint choices.
- Make `aigw verify` use the configured Codex executable for its Codex request,
  through one selected synchronized target and without persisting a session.
- Report the measured Codex executable version and SHA-256 identity without
  exposing credentials or response content.
- Keep operator history and external gateway lifecycle outside AIGW ownership.
- Reuse the existing configuration, credential, projection, process, and verify
  authorities; introduce no parallel verifier or lifecycle state.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Define endpoint-neutral composition and require
  real-client verification of an AIGW-projected Codex route.

## Impact

- `openspec/specs/product-control-plane/spec.md`
- `internal/codex`
- `internal/verification`
- `internal/cli/verification`
- Focused acceptance tests and `CONTRIBUTING.md`
- No new runtime dependency, configuration authority, or provider-specific
  production branch
