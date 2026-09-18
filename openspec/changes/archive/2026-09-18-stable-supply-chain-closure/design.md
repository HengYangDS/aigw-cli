## Context

The canonical specification already requires the latest stable
repository-owned supply chain. This Change refreshes that contract without
creating another version authority. Each ecosystem retains its native owner:
Go modules, npm, Mise, CUE CI declarations, and pinned OCI or Action identities.

## Goals / Non-Goals

**Goals:**

- Establish current stable versions from official release sources.
- Keep one direct version owner per dependency and deterministic generated
  closure.
- Preserve supported runtimes, platforms, product behavior, and the installed
  rollback boundary.
- Prove source, native, and release construction after the graph changes.

**Non-Goals:**

- Product features, configuration migration, credential access, publication,
  or installed-runtime replacement.
- A second dependency manager, update ledger, compatibility path, or hand-made
  transitive pin.

## Decisions

Use the native resolver for each ecosystem. Update direct declarations first,
regenerate locks and CUE projections through their existing commands, then run
the same resolution again and require no byte change. A reported transitive
update is not promoted to a direct dependency unless the source owns it.

Treat an official non-prerelease release as stable, subject to the repository's
supported runtime and platform constraints. A major update is admitted only if
the complete compatibility gates pass; otherwise retain the supported major
and record the exact incompatibility in this Change.

Mise 2026.9.11 deliberately stops writing `provenance_verified`; the retained
`provenance` field identifies a verification method used while generating the
lock, while checksums bind later installs. The repository test follows that
current contract instead of requiring inert legacy metadata.

The npm resolver would otherwise select `whatwg-url` 17.1.1, whose published
metadata points at an attestation endpoint that currently returns 404. Retain
17.1.0 within jsdom 30.1.0's declared range because its attestation is retrievable;
this exception expires when a newer fully attestable compatible release exists.

An isolated GoReleaser 2.18.2 probe changes a signed binary's timestamp after
the build hook and observes the declared `builds_info.mtime` in the archive.
The former Node timestamp rewrite is therefore obsolete and is removed.

## Verification

1. Record current official versions and selected compatible versions.
2. Refresh direct declarations, locks, and generated projections.
3. Run bootstrap, dependency policy, focused resolver checks, the complete
   source gate, native acceptance, and release construction.
