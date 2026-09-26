# Proposal

## Why

AIGW creates a fresh Hermes home for each live verification, then invokes
`hermes --version`. The installed Hermes checks GitHub for updates on that path;
an isolated release-client journey failed at the version probe after an Account
rename. Verification must not depend on an unrelated update service or conceal
whether its own probe failed or timed out.

The release journey also exposed a dependency cycle: exact artifact acceptance
required a stable tag, but the tag cannot precede candidate acceptance and
Change archive. Candidate trust must bind to the signed source commit without
weakening tag-bound publication.

The current Change task also combines local source acceptance with every peer's
later CI. Checking that task changes the commit it claims to verify, while
archiving changes it again. Pre-archive acceptance and post-archive delivery
must have separate owners without dropping either obligation.

## What Changes

- Disable Hermes passive update checks in AIGW's disposable verification home
  using Hermes' native configuration. Do not change the user's Hermes home,
  selected Model, Route, or credential source.
- Report a bounded version-probe failure by cause category without exposing
  vendor stderr, credentials, or private paths.
- Add a focused regression and repeat real-client acceptance before any remote
  release. The unpublished local `v0.3.2` tag remains immutable; the repaired
  product receives a new version identity.
- Admit an explicitly selected untagged artifact candidate against signed HEAD
  and independently trusted artifact provenance. Keep release verification and
  publication tag-bound; never mint a temporary tag to make acceptance pass.
- Close implementation and candidate acceptance in official Change tasks before
  archive. Keep each peer's accepted-ref and tag CI, published assets,
  installation, and retirement as independently proved post-archive obligations
  in canonical specifications and Change design.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Hermes verification must use only its declared
  client and selected inference endpoint, without an unrelated update request;
  pre-tag artifact acceptance must not weaken signed-tag publication, and
  local source acceptance must not depend on a later peer delivery.
- `product-quality`: Change tasks must close pre-archive implementation and
  candidate acceptance without treating archive as proof of later delivery.

## Impact

The Hermes adapter, release provenance verifier, native client gate, version
metadata, release evidence, and Change lifecycle contracts are affected. No
new dependency, daemon, proxy, credential store, or client configuration owner
is added.
