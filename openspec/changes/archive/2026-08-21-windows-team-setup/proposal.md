## Why

Team setup currently treats every Account in a reviewed manifest as immediately
required and couples manifest import, credential enrollment, client discovery,
projection, and live validation into one all-or-nothing operation. This prevents
a user with one valid provider Token—or with Claude Code or Codex not yet
installed—from adopting the team configuration even though those states are
independent and can converge later.

## What Changes

- Treat a team manifest as a credential-free capability catalogue, not an
  activation mandate for every Account.
- Allow setup to succeed with zero connected Accounts and zero installed
  clients while reporting the exact incomplete state and next useful actions.
- When one or more Account Tokens are available, select usable routes from
  those Accounts and validate only the installed clients that consume them.
- Configure only clients discovered on the current host; a later repair
  rediscovery adopts a newly installed Claude Code or Codex client without
  re-importing the manifest.
- Make human and machine output distinguish imported, connected, selected,
  projected, ready, and deferred states.
- Extend native Windows acceptance from executable lifecycle smoke to the same
  user journey exercised on other supported hosts.
- **BREAKING**: remove the interpretation that `setup --from` requires a Token
  for every manifest Account or that every recommended client must already be
  installed.

## Capabilities

### New Capabilities

- `progressive-team-onboarding`: Incremental team-manifest adoption across
  catalogue import, Account connection, Route selection, client discovery, and
  later convergence.

### Modified Capabilities

- `product-control-plane`: Clarify that credentials and client projections are
  optional local activation state, independent from token-free manifest data.

## Impact

- Affected owners: configuration manifest semantics, onboarding, Route
  selection, recovery rediscovery, readiness presentation, documentation, and
  native acceptance.
- AIGW continues to own only configuration, local Account credentials, Routes,
  and explicit client projections. It does not install clients, manage an
  external Responses service, or carry model traffic.
- The reviewed team manifest may select any conforming Responses endpoint.
  Endpoint availability is therefore a conditional Route dependency, never an
  AIGW installation dependency or a named product integration.
- No compatibility command or parallel setup path is retained; the existing
  `setup`, `use`/`rotate`, `repair`, and `check` journey becomes progressive.
- Existing mature prompt, secret-store, discovery, synchronization, and
  presentation owners are reused; no new framework or state store is added.
