# DR-0002: GitHub as an Independent Release Peer

- Status: superseded by [DR-0007](dr-0007-local-first-independent-forge-release.md)
- Date: 2026-07-17
- Owner: Release maintainers

## Context

The original decision established that GitHub release trust could not depend on
GitLab availability or a host-specific immutability claim. That independence
remains correct, but allowing a Forge to construct its own tag created
different product objects for the same release.

## Decision

GitHub remains an independent public distribution peer. It now receives the
same locally signed commit and annotated tag object as every selected peer.
GitHub independently runs hosted CI, creates its Release record, publishes the
asset matrix, and verifies its checksums. It neither downloads from GitLab nor
constructs a GitHub-specific product object.

Transport credentials and GitHub's account-level `Verified` presentation do not
own product identity. The exact local object and explicit product trust anchor
do.

## Consequences

GitHub remains independently releasable and auditable, but it no longer owns a
distinct commit or tag identity. Existing formal release records remain
historical evidence; duplicate local tag aliases are not product authority.
All new formal releases use exact local product objects.

## Revisit Trigger

Revisit only if GitHub stops being a supported publication peer or the product
ceases to use Git object identity as its release authority.
