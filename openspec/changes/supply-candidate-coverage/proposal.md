## Why

The repository's candidate calculation decides whether its supply chain is
current. Since `aa55e2fc` (2026-10-26) moved the native client pins from
package-name strings into the CUE `nativeClients` block, the Renovate rule for
pinned npm clients matches no input, and no rule has ever read the Hermes pin.
The calculation therefore reports no Codex, Claude Code or Hermes release: on
2026-11-03 it described the supply chain as current while Hermes v0.22.6
(2026-11-02T17:18Z) was available. uv 0.13.7 followed on 2026-11-03T02:45Z; it
embeds rustls 0.23.46, which fixes GHSA-c8m5-c952-6cm4 and three low-severity
rustls advisories that affect the 0.23.45 of uv 0.13.6.

## What Changes

- The Renovate policy reads the Codex, Claude Code and Hermes pins of the CUE
  `nativeClients` block from their publishers' npm packages and GitHub tags,
  and the rule for the retired package-name strings is removed.
- A dependency test fails unless exactly one candidate rule reads each native
  client pin with its pinned value.
- Advance uv to 0.13.7 and the Hermes pin to v0.22.6: its tag commit and the
  SHA-256 of that release's `scripts/install.sh`.
- Claude Code 2.1.317 stays held until its release age ends at
  2026-11-06T00:04Z, three days after its last supplying platform package.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: the candidate calculation reads every pin of the CUE
  authority, so the supply chain is described as current only after each pin
  is audited.

## Impact

The Renovate policy, the CUE authority, the Mise declaration and lock, both
Forge projections, and the dependency and projection tests change. The native
client journeys run with Hermes v0.22.6. The released AIGW programs, user
configuration and credentials do not change, so no release follows.

The CUE authority stays the only owner of each pin, and the Renovate policy the
only owner of candidate rules; the change reuses Renovate's regex managers and
adds no tool. Out of scope: the proxy repository's candidate calculation, a
change to the release-age policy, and Claude Code 2.1.317 before its release
age ends.
