## Why

On 2026-10-29 the user asked for every supply input to move to its latest
stable release. A Renovate `dryRun=full` candidate calculation with the
repository policy on GitHub `dev` (`12740c4e`, 2026-10-29) reached every
datasource and admits nine updates. Go 1.27.3 and every direct Go module are
already the latest stable releases.

GHSA-gg6h-894w-4m9c (CVE-2026-116795) is a high-severity, quadratic-time
lookbehind denial of service in re2js `>=2.2.0 <2.8.7`. markdownlint 0.42.0,
the newest release, pins re2js 2.8.6 exactly, so `npm audit` reports both
packages. The newest native client releases past the three-day release age,
Codex 0.164.0 and Claude Code 2.1.310, are also ahead of their pins.

## What Changes

- Advance Node to 26.13.0, uv to 0.13.6, gh to 2.103.3, glab to 1.126.0 and
  the Renovate image to 44.192.2 in the Mise declaration, and regenerate their
  lock entries.
- Advance `jdx/mise-action` to v5.2.4 and the Mise image to 2026.10.22 in the
  CUE authority, with that release's Windows ARM64 executable and shim
  digests and the Mise floor, then regenerate both Forge projections.
- Advance OpenSpec to 1.15.0 and the katex override to 0.19.1, refresh the
  npm lock within its declared ranges, and override markdownlint's re2js with
  2.9.1 until a markdownlint release requires a fixed re2js.
- Pin Codex 0.164.0 and Claude Code 2.1.310 for the native client journeys.
  Hermes v0.22.5 is still the latest release.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: a direct dependency's exact pin of a vulnerable
  release is overridden with a fixed release until the dependency requires
  one.

## Impact

The Mise declaration and lock, `package.json` and `package-lock.json`, the CUE
authority, the retained-step digests and both generated Forge projections
change. The GitLab Windows runner's Mise and the GitLab Windows client
supplier archive follow at the deferred GitLab catch-up, before any aigw
GitLab job consumes this declaration. The AIGW program, its release artifacts,
user configuration and credentials do not change, so no release follows.
