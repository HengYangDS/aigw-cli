## Why

GitHub advisory GHSA-hq85-5cpj-mr62 (critical, published 2026-10-20) reports
that uv before 0.13.4 trusts, in its bundled certificate store, an emSign root
whose intermediate CA keys are compromised. AIGW locks uv 0.13.1 in Mise for
native client acceptance, so every local and Forge job that installs a client
through uv could accept a certificate from that compromised chain. uv 0.13.4,
a stable release published the same day, is the first fixed version.

## What Changes

- Advance the Mise-locked uv from 0.13.1 to 0.13.4 and regenerate its
  platform entries in the Mise lock with Mise's own `mise lock`.
- Add a scenario to the supply-chain requirement: an advisory against a locked
  tool advances it to the first fixed stable release without waiting for the
  ordinary release-age delay, which the repository's Renovate policy already
  sets to zero days for vulnerability fixes.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: the latest-stable supply-chain requirement gains a
  scenario for a security advisory against a locked tool.

## Impact

The Mise declaration and lock change. uv is an acceptance tool, not an AIGW
runtime dependency, so product code, released binaries, installed AIGW, user
configuration and credentials do not change. ETHOS's own runtime bundles uv
separately; rebinding to a fixed ETHOS runtime is outside this Change.
