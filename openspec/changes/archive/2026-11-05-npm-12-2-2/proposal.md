## Why

npm 12.2.2, published 2026-11-04T23:02Z, bundles undici 6.29.1 and ip-address
10.7.5. The npm 12.2.1 that aigw locks as its repository npm bundles undici
6.29.0, which five advisories name (GHSA-2m5v-pch6-74hx, GHSA-4qv8-2p9j-2685,
GHSA-9fm3-97px-rjf6, GHSA-qrxm-fqhp-jwvm, GHSA-xjj6-f33q-929c), and ip-address
10.7.2, which GHSA-rxp7-6hxx-x8q5 names. OSV reports none of them for the
bundled releases of 12.2.2. A security fix moves a locked tool to its first
fixed release without the minimum release age.

## What Changes

- Advance the repository npm from 12.2.1 to 12.2.2 in the Mise declaration,
  its lock and its aube dependency graph.
- Keep the two remaining advisories of the npm bundle, in glob 13.0.6 and
  minimatch 10.2.5, disclosed until an npm release fixes them.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: an advisory in a dependency that a locked tool bundles moves
  the tool like an advisory that names the tool itself.

## Impact

The Mise declaration, the Mise lock and the npm aube lock change. The released
programs, user configuration and credentials do not change, so no release
follows.
