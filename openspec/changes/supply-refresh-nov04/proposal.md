## Why

The user directs that every supply chain move to its latest stable release.
On 2026-11-04 at 10:41 CST, a Renovate `dryRun=full` candidate calculation
with the repository policy on `c778fe12` admits three updates:

- the Renovate image 44.210.0, published 2026-11-03T18:47Z;
- glab 1.128.0, published 2026-11-03T12:18Z;
- Hermes v0.22.7, published 2026-11-03T21:53Z.

The calculation also reports the GitLab Mise image 2026.11.0 and holds Claude
Code 2.1.318 until its three-day release age ends.

## What Changes

- Advance the Renovate image to 44.210.0, pinned to its index digest.
- Advance glab to 1.128.0 for all six locked platforms.
- Advance the Hermes native client pin to v0.22.7: its tag commit and the
  unchanged SHA-256 of its `scripts/install.sh`.
- Keep Mise at 2026.10.25. One CUE value selects the Mise release for the
  GitLab image, the GitHub mise-action and the Windows runner-owned executable.
  Under 2026.11.0, the safe and locked resolution that ETHOS proofs use refuses
  aigw's tools, because safe mode no longer trusts the project lockfile: in a
  fresh data directory, core tools such as Node fail, and so does `npm:npm`.
  The move waits until a supported fix lets that resolution pass.
- Keep Claude Code at 2.1.316 until 2.1.318's release age ends.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: a stable release that breaks a declared resolution mode of
  the repository's own verification stays held, with its owner and exit
  condition named, instead of being taken or worked around.

## Impact

The following change: the Mise declaration and lock, the CUE native-client
pin, both Forge projections, the projection tests, and the Windows client
supplier name if it embeds the Hermes version. The released AIGW programs,
user configuration and credentials do not change, so no release follows.
