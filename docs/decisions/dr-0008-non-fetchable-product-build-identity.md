# DR-0008: Use a Non-Fetchable Product Build Identity

- Status: accepted
- Date: 2026-08-07

## Context

AIGW is distributed as executables and native packages, not as a public Go
library. A remote, personal, private-network, or local-filesystem module path
would falsely make deployment topology part of the product API.

## Decision

The Go module path is the non-fetchable build identity `aigw-cli`. Reusable
implementation remains below `internal/`. Product source does not encode a Git
remote, organization, homepage, local path, or personal namespace.

A public Go package requires a separate decision and a real stable,
organization-owned, resolvable module path. It is not admitted as a workaround
for toolchain expectations.

## Consequences

The repository remains portable across GitLab, GitHub, local checkouts, and
future hosting changes. Consumers install released products rather than import
unstable internal packages.

## Revisit Trigger

Revisit if AIGW deliberately publishes a supported Go library API with an
organization-owned module namespace and compatibility contract.
