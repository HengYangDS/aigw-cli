# DR-0007: Keep Local Closure and Independent Forge Publication

- Status: accepted
- Date: 2026-08-07

## Context

GitLab and GitHub have separate identities, trust inputs, CI, tags, Release
records, and outages. Coupling either publication to the other would create a
common failure path and make local development depend on remote availability.

## Decision

Local source can build, test, package, install, and verify without a Forge.
GitLab and GitHub independently project and sign provider-native histories and
publish complete release assets. Neither pipeline queries, authenticates to,
downloads from, or publishes through the other.

When both publications are present, a read-only audit compares their source
semantics and common asset bytes. One-sided availability may supply a fully
verified update, but it never proves dual publication.

## Consequences

Either release plane remains useful during a peer outage. Publication actors,
emails, signers, keys, fingerprints, credentials, and coordinates stay in the
protected execution context rather than product source.

## Revisit Trigger

Revisit if a Forge is retired or one external release authority formally
replaces both provider-native publication domains.
