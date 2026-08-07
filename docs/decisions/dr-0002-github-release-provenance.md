# DR-0002: Accept Signed GitHub Release Provenance

- Status: accepted
- Date: 2026-07-17
- Owner: Release maintainers

## Context

The GitHub publication peer is public. GitHub host rules are useful hardening,
but they are not portable release evidence and may differ from GitLab controls.

## Decision

Use GitHub as the public distribution peer. GitHub release tags are accepted as
signed, independently verified provenance records rather than delegating trust
to a host-specific ruleset. AIGW automation must continue to avoid copying,
overwriting, deleting, or regenerating a provider-native release tag.

## Consequences

GitHub release acceptance requires the current remote tag to verify against the
protected GitHub trust input and the complete GitHub artifact matrix to pass
its own checksums and installation tests. A GitHub release never waits for or
downloads from GitLab. When both releases exist, a separate read-only audit may
establish cross-Forge asset parity; that audit is evidence about two completed
publications, not an input to either publication.

A manual GitHub tag mutation remains possible at the hosting layer and is
treated as a detected provenance failure, not as an impossible state. No
release or host-route claim may state that the GitHub tag is host-enforced
immutable.

## Revisit Trigger

Revisit this decision only if the product stops using GitHub as a public
distribution peer or the cross-Forge provenance model changes.
