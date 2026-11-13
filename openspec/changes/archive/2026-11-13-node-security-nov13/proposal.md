## Why

Node.js 26.13.1, the security release of the 26 LTS line dated 2026-11-10,
fixes 16 vulnerabilities, seven of them rated High: CVE-2026-123863,
CVE-2026-123864, CVE-2026-123865, CVE-2026-123866, CVE-2026-123872,
CVE-2026-123874 and CVE-2026-126051, mostly in http2 and tls. The repository
runs its Markdown, OpenSpec and audit tooling on Node 26.13.0.

The user directs that every supply chain move to its latest stable release. On
2026-11-13 at 09:54 CST, the Renovate candidate calculation with the
repository policy finds 18 updates. One of them is wrong: it proposes the Hermes
canary tag `v0.23.1+canary.20261110T064422Z` instead of the stable release
v0.23.1. The rule parses Hermes tags with `semver-coerced` versioning, which
drops the `+canary…` build metadata, so the canary tag passes as a release.

## What Changes

- Advance Node to 26.13.1 for all six locked platforms.
- Advance the Mise-locked tools: gh 2.104.0, glab 1.130.0, goreleaser 2.20.2,
  OSV Scanner 2.7.1, Syft 1.57.1, typos 1.52.2 and uv 0.13.8.
- Advance the Go modules of the released program: `charm.land/lipgloss/v2`
  2.1.0, `golang.org/x/crypto` 0.59.0 and `golang.org/x/term` 0.48.0, with
  `golang.org/x/text` 0.44.0 beneath them. x/crypto 0.59.0 no longer contains
  the unmaintained `openpgp` package that GO-2026-5932 names; AIGW imports only
  `golang.org/x/crypto/ssh`.
- Advance markdownlint to 0.42.1, and the jsdom that `@mermaid-lint/core` uses
  to 31.0.0.
- Advance mise-action to 5.2.8 and the CI's Hermes to the stable release
  v0.23.1, whose installer is unchanged.
- Advance the Renovate image to 44.236.5, pinned to its index digest, and write
  `dependencyDashboard` in the object form that its strict validation requires.
- Parse Hermes tags as release tags only, so the candidate calculation no longer
  reports a canary tag.
- Hold, each for a stated reason:
  - Mise 2026.11.7 and the GitLab Mise image, which need the Windows runner's
    Mise replaced in a guest window;
  - `go.yaml.in/yaml/v4`, a new major release with a new import path that 31
    files use;
  - Codex 0.167.x and Claude Code 2.1.32x, which move together with one Windows
    supplier archive after Codex 0.167.2's release age ends at
    2026-11-14T22:50Z;
  - Prettier 3.10.1 and re2js 2.10.0, inside their release age.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: the candidate calculation does not report a client tag that
  is not a stable release.

## Impact

The Mise declaration and lock, `go.mod` and `go.sum`, the npm manifest and lock,
the CUE authority with its generated workflows, and the Renovate policy change.
The released program takes new Go modules, so the next release carries them;
user configuration and credentials do not change.
