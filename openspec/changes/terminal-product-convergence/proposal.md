# Proposal

## Why

AIGW is a local control plane, but its current setup, readiness, credential
entrypoint, client admission, and release paths do not yet form one dependable
cross-platform user journey. A token-free import currently recommends `aigw
sync` even when no client can be selected; that dry run then suggests one
provider's Token although any compatible Account is optional. The fixed-path
credential copy also survives package replacement without a defined successor
cutover. Passing source checks cannot settle these installed-product contracts.

## What Changes

- Make Account, Route, Client Binding, credential availability, projection, and
  live verification distinct typed states with one next-action decision across
  setup, use, sync, check, doctor, recovery, and machine output. Preserve an
  explicit selection; do not turn a recommendation into a required provider.
- Admit Provider models by declared protocol and tested capability, not a
  growing family of name-specific branches. Maintain the team catalogue from
  reviewed upstream evidence and qualified live probes; expose only client
  choices that a real native client can use.
- Define one client-adapter contract for ownership, native configuration,
  credentials, merge, rollback, withdrawal, and real tool-loop evidence. Extend
  beyond current clients only when that entire contract is implementable.
- Replace the fixed-path, immutable-but-never-upgraded credential copy with a
  cross-platform successor cutover that preserves retained callers until their
  exact consumer is gone. Do not add a daemon, shell wrapper, or second secret
  store to hide package-manager replacement.
- Close the macOS, Linux, and Windows journeys for locked bootstrap, build,
  install, setup, projection, update, rollback, uninstall, and real clients.
  Keep AIGW independent of any optional external Responses Proxy.
- Converge quality, supply-chain, CI, release, documentation, and physical
  repository ownership on their existing native authorities. Delete parallel,
  obsolete, unconsumed, or misleading surfaces instead of preserving them with
  compatibility shims. Project one CUE CI intent to GitHub and GitLab while
  proving each selected peer's own admission and assets.

**BREAKING:** Retire obsolete configuration, helper, output, and repository
surfaces only after their current consumers and migration boundary are proved.
Historical Git and immutable evidence remain history, not active compatibility
paths. This Change does not claim that every exploratory Provider or Agent
client will be supported.

## Capabilities

### Modified Capabilities

- `progressive-team-onboarding`: Optional credentials and clients, deferred
  setup, and a next action that is usable in the current state.
- `route-client-selection`: Explicit selection, provider recommendations, and
  advisory Provider preference without a false request-time failover claim.
- `secret-storage`: Portable credential access and safe versioned entrypoint
  lifecycle without unproved authorization.
- `product-control-plane`: Independent Client Adapters use a qualified
  credential reader rather than a package-manager-owned CLI path.

The existing `cli-readiness`, `projection-format`, `release-distribution`,
`ci-diagnostics`, `product-quality`, and `repository-organization` specifications
already state the required behavior. Their implementation, evidence, and
obsolete carriers will be converged without restating those requirements or
creating a second specification authority.

## Impact

The Change may alter `internal/`, `cmd/`, `tools/`, `manifests/`, `.config/`,
locked dependencies, both generated Forge workflows, tests, and canonical
documentation. Current 0.3.1 users, explicit route choices, native credentials,
running clients, and external Proxy installations remain protected during
development. An active P0 incident may be repaired first, but a source-green
or partially published candidate is not terminal acceptance.

Request-time transparent Provider failover is outside AIGW's control plane:
only a separately selected data-plane service or a native client with a proven
contract can observe and retry live API traffic. AIGW may declare preferences
and present safe explicit switching, but must not pretend to provide request
failover without owning that observation boundary.
