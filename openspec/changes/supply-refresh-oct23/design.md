# Design

## Context

Mise declares the developer, CI and release tools in `.config/mise/config.toml`,
and `mise lock` owns their lock entries. The CUE authority
`.config/ci/pipeline.cue` pins GitHub Actions by commit and the Windows native
client releases; both Forge projections are generated from it. The Renovate
policy admits checksummed Mise tools and digest-pinned CI inputs without the
release-age delay and keeps npm's three-day delay, which also governs the
Windows native client pins.

## Goals / Non-Goals

**Goals:** take every update the candidate calculation admits for the Mise
tools and CI inputs, including the Node and uv security releases.

**Non-Goals:** the Windows native client pins, the Go modules, the Mise image
and its Windows runner, the Mise minimum version, and any change to AIGW
programs, credentials, configuration or client projections.

## Decisions

### Tools advance through their declarations

Each tool moves in its single declaration: Node, npm, uv, gh, GoReleaser and
the Renovate image in Mise, and mise-action in the CUE authority. `mise lock`
regenerates the lock entries for every locked platform, and the projector
regenerates the GitHub and GitLab files, so no version literal is edited twice.

### Client pins move together after the release age

Claude Code 2.1.304 has passed npm's three-day release age, but Codex 0.164.0,
published on 2026-10-23, passes it only on 2026-10-26. Each pin change needs a
Windows native client journey, so both move in one later Change rather than in
two journeys three days apart. The next candidate calculation on or after
2026-10-26T04:52Z lists them again.

### Unreachable datasources leave inputs unaudited

From this workstation the Go module proxy timed out and Docker Hub's token
service refused the Mise image lookup. Renovate logged a failed lookup for each
of those dependencies and proposed nothing for them, which does not show that
they are current. They stay unaudited until a calculation reaches their
datasources. The Mise image also waits for the GitLab Windows runner
replacement.

## Risks / Trade-offs

- [A tool release changes behavior] → the dependency tasks, `mise run check`
  and the full proof run before integration, and the GitHub native jobs run on
  every platform.
- [GoReleaser 2.19.0 packages releases differently] → the release tests build
  a snapshot with the locked GoReleaser, and the next release keeps its
  construction and verification steps.

## Migration Plan

Mise installs the new tools on the next bootstrap of each checkout and runner.
Nothing migrates, and installed AIGW is unaffected. GitLab receives the change
with the pending GitLab catch-up.

## Open Questions

- None.
