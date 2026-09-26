# Proposal

## Why

AIGW creates a fresh Hermes home for each live verification, then invokes
`hermes --version`. The installed Hermes checks GitHub for updates on that path;
an isolated release-client journey failed at the version probe after an Account
rename. Verification must not depend on an unrelated update service or conceal
whether its own probe failed or timed out.

## What Changes

- Disable Hermes passive update checks in AIGW's disposable verification home
  using Hermes' native configuration. Do not change the user's Hermes home,
  selected Model, Route, or credential source.
- Report a bounded version-probe failure by cause category without exposing
  vendor stderr, credentials, or private paths.
- Add a focused regression and repeat real-client acceptance before any remote
  release. The unpublished local `v0.3.2` tag remains immutable; the repaired
  product receives a new version identity.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Hermes verification must use only its declared
  client and selected inference endpoint, without an unrelated update request.

## Impact

The Hermes verification adapter, its focused tests, the real-client release
gate, version metadata, and release evidence are affected. No new dependency,
daemon, proxy behavior, credential store, or client configuration owner is added.
