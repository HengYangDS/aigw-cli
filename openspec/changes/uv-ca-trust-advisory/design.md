# Design

## Context

`.config/mise/config.toml` declares uv for native client acceptance, and
`.config/mise/mise.lock` holds each platform's release URL, checksum and
GitHub attestation provenance. No CI projection repeats the uv version.

## Goals / Non-Goals

**Goals:** every local and Forge job that installs uv resolves the first
release outside the advisory's vulnerable range.

**Non-Goals:** rebinding ETHOS's own runtime, which bundles its own uv, or
changing any other locked tool.

## Decisions

### Advance to the first fixed stable release

uv 0.13.4 is the first release outside the vulnerable range of
GHSA-hq85-5cpj-mr62 and is the latest stable release. The repository's
Renovate policy sets `minimumReleaseAge` to zero days for vulnerability fixes
and for Mise-managed tools, so the advance needs no exception.

### Mise owns the lock

`mise lock` regenerates the uv entries from the upstream release, including
checksums and provenance; the lock is not edited by hand.

## Risks / Trade-offs

- [A uv release behaves differently in client acceptance] → the full proof and
  the native jobs on every platform run the acceptance with the new uv before
  integration.

## Migration Plan

Mise installs uv 0.13.4 on the next bootstrap of each checkout and runner; no
state migrates.

## Open Questions

- None.
