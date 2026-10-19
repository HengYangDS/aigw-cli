## Why

Mise 2026.10.14's Windows release ships a `mise-shim.exe` without the version
tag that its own `mise.exe` checks, so every native Windows job, on the
self-hosted runner and on GitHub's hosted runner, logs that the shim is older
than Mise. Mise 2026.10.15 fixes the release (jdx/mise#14656). A new Renovate
candidate calculation on `d02033e0` also admits three more stable updates.

## What Changes

- Advance the CUE-pinned Mise image to 2026.10.15 with the runner-owned Windows
  ARM64 executable and shim digests from its verified official archive, and
  `jdx/mise-action` to v5.2.1; regenerate both peers' projections.
- Advance OpenSpec to 1.14.3, which no longer pulls `braces` itself, and the
  Renovate image to 44.168.1.
- Keep the Windows native client pins; they still need their own admission.
- Replace the Windows runner's Mise before GitLab consumes the declaration, as a
  post-archive rollout step.

## Capabilities

### Modified Capabilities

- `product-control-plane`: the latest-stable supply-chain requirement gains a
  scenario for an admitted tool release that fails its own consistency check.

## Impact

Dependency declarations, the npm lock, the generated CI projections and the
Windows CI runner's Mise change. Product code, the release matrix, installed
AIGW, user configuration and credentials do not.
