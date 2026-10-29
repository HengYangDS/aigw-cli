# Design

## Context

Mise declares the developer and CI tools, and `mise lock` owns their lock
entries. The CUE authority `.config/ci/pipeline.cue` pins GitHub Actions by
commit, the Mise image by digest, the runner-owned Windows ARM64 Mise by
executable and shim digest, and the native client releases. `tools/ci
project` generates both Forge files from it, and
`.config/ci/retained-steps.toml` records the digest of each retained
operating-system step. npm owns `package-lock.json` from `package.json`,
including its `overrides`.

The candidate calculation ran the pinned Renovate image with
`RENOVATE_DRY_RUN=full` on GitHub `dev` (`12740c4e`) and reached every
datasource, Go modules included. Its evidence and the release facts below are
in the main checkout's `build/evidence/supply-chain-20261029/`.

## Goals / Non-Goals

**Goals:**

- Take every update that the candidate calculation admits.
- Remove the re2js advisory from the lock without replacing markdownlint.
- Pin the newest native client releases past the release age.

**Non-Goals:**

- Raising a transitive Go module that its direct dependency does not select.
- A transitive npm package release outside the range its dependents declare.
- The GitLab Windows runner's Mise and the GitLab Windows client supplier
  archive, which the deferred GitLab catch-up owns.
- Any change to AIGW programs, credentials, configuration or client
  projections.

## Decisions

### Tools advance through their declarations

Each tool moves in its single declaration. `mise lock` regenerates the entries
of Node, uv, gh and glab for all six locked platforms, and `tools/ci project`
regenerates both Forge projections.

- The Renovate image is pinned by its index digest
  `sha256:37960a62ba1faa9079a7c4eead9b3f4470748a64ecd207e06c09fd8858673e17`,
  read from `ghcr.io` with `docker buildx imagetools inspect`.
- mise-action v5.2.4 is pinned by commit
  `6be3c7ed4c21d4f76b208d351d95aac273ebb76d`, the target of the upstream
  `v5.2.4` tag.
- The Mise image `2026.10.22-debian` has the index digest
  `sha256:d54a295f3df1d041e289e7643655ff771df15ce74eece5c62ef72b41c9d9a132`
  on Docker Hub. The official `mise-v2026.10.22-windows-arm64.zip` matches
  `SHASUMS256.txt` (`0c268c9d…f4a2`); its `mise.exe` is
  `6024bc1aaf5b0bb3206572f91a530f907cbbfa81f7313423d2af1778971eaa6a` and its
  `mise-shim.exe` is
  `4dd22a62c218ac92d2e7c205cd38c34d52dd7ccbc51e7c5fa060802dbb6610dd`.
- The Mise floor moves with the image, as it did for 2026.10.19.

### npm refreshes the lock within its declared ranges

`npm update --package-lock-only --ignore-scripts` with the locked npm and
`--before` set three days back moves every transitive package to the newest
release that its dependents admit and that is past the release age, as the
candidate calculation's own lock regeneration does. A release outside a
dependent's range waits for that dependent.

### The override takes the highest fixed re2js

markdownlint 0.42.0 declares `re2js: 2.8.6` exactly, and no markdownlint
release requires a fixed one. An `overrides` entry scoped to markdownlint
replaces that pin, so no other package's resolution changes. The repository
policy lets a security fix bypass the release age and takes the highest fixed
version, which is re2js 2.9.1. It has no dependencies or lifecycle scripts and
requires Node 20 or later.

Two alternatives were rejected:

- markdownlint 0.41.1, which `npm audit` proposes, gives up the current
  release's rules.
- re2js 2.8.8, the smallest fixed step, is not the highest fixed release that
  the policy selects.

The override leaves when a markdownlint release requires re2js 2.8.7 or later
(DavidAnson/markdownlint#2338).

### Native client pins move with their journeys

Codex 0.164.0 (published 2026-10-23T04:52Z) and Claude Code 2.1.310
(2026-10-23T21:20Z) are the newest stable releases past the three-day release
age; Codex 0.165.0 and Claude Code 2.1.312 are not yet past it. The CUE
authority pins them once, and the Windows supplier name follows. Hermes
v0.22.5 is the latest Hermes release, so its commit and installer digest stay.

### GitLab follows at its catch-up

aigw's GitLab peer is held at v0.4.8 until ETHOS can publish the historical
tags. No aigw GitLab job runs before then. The catch-up replaces the runner's
Mise and builds the Windows client supplier for each declaration it runs,
which is when "A runner-owned tool advances with its declaration" applies.

## Risks / Trade-offs

- **A tool release might change behavior.** The dependency tasks,
  `mise run check` and the full proof run before integration, and the GitHub
  native jobs run on every platform.
- **re2js 2.9.1 is newer than the release markdownlint tested.** The Markdown
  checks run markdownlint over every Markdown input before integration.
- **A client release might change its package layout.** The native client
  journeys install and run the clients on Linux, macOS and Windows before
  integration.

## Migration Plan

Mise installs the new tools on the next bootstrap of each checkout and runner.
Before the forges integrate this Change, the GitHub native client journeys of
Linux, macOS and Windows pass with the new pins on the proposal. Installed
AIGW is unaffected.

## Open Questions

- None.
