## Context

This lane first took Node 26.13.1 alone, as a Change without spec deltas. The
repository gate fails on every OpenSpec finding, including the `INFO` that
OpenSpec 2.0 reports for `skip_specs`, so a Change here needs a delta. The
candidate calculation of 2026-11-13 then found both the routine updates and a
defect in one of its own rules, so this Change became the refresh of that day.
It keeps its name, because the lane's first commit already names it.

## Goals / Non-Goals

**Goals:**

- Take every stable release whose admission evidence passes at the 09:54 CST
  cut-off of 2026-11-13.
- Stop the candidate calculation from reporting a client build that is not a
  release.

**Non-Goals:**

- Moving Mise outside a Windows runner window.
- Migrating to `go.yaml.in/yaml/v4`.
- Building a Windows supplier archive for one client while the other is held
  by its release age.

## Decisions

### Parse Hermes tags with release-only versioning

The Hermes rule reads the release tag that follows the pinned commit. With
`semver-coerced` versioning, Renovate coerces `v0.23.1+canary.20261110T064422Z`
to 0.23.1 and offers it as an update. Regex versioning that accepts only
`v<major>.<minor>.<patch>` makes every other tag invalid, so Renovate ignores
it. The alternative, an `allowedVersions` pattern in a separate package rule,
would put one client's admission in two places.

### Hold the native clients together

GitLab's Windows job installs Codex and Claude Code from one reviewed supplier
archive named after both versions. Claude Code 2.1.321 is admissible now, but
Codex 0.167.1 and 0.167.2 are not. Moving Claude Code alone would need one
archive now and another in two days, so both move after Codex 0.167.2's age
ends.

### Verify locked checksums against the publishers' lists

Each Mise lock entry that changes is compared with the checksum list that its
release publishes: `SHASUMS256.txt` for Node, `checksums.txt` for glab on
GitLab, and each GitHub release's own list together with GitHub's asset
digest. typos publishes no list, so its entries are compared with the asset
digests alone. A mismatch stops the refresh.

## Risks / Trade-offs

- Lip Gloss 2.1.0 judges dark and light colors by CIELAB lightness instead of
  HSL. AIGW styles its output with fixed 256-color values and no adaptive
  color, so its output does not change; the rendering tests confirm it.
- jsdom 31 is a major release that `@mermaid-lint/core` 0.53.1 does not declare.
  The Mermaid gate over the repository's documents is its acceptance; if it
  fails, jsdom stays at 30.1.3.
