---
subject: aigw:decision:0008
role: decision
state: canonical
decision_id: DR-0008
decision_status: accepted
relations:
  canonical_for: Use a Non-Fetchable Product Build Identity
---

# DR-0008: Use a Non-Fetchable Product Build Identity

- Date: 2026-08-07

## Context

AIGW is distributed as executable archives, not as a public Go
library. A remote, personal, private-network, or local-filesystem module path
would falsely make deployment topology part of the product API.

## Decision

The Go module path is the non-fetchable build identity `aigw-cli`. Reusable
implementation remains below `internal/`. Product source does not encode a Git
remote, organization, homepage, local path, or personal namespace.

A public Go package requires a separate decision and a real stable,
organization-owned, resolvable module path. It is not admitted as a workaround
for toolchain expectations.

## Alternatives Rejected

- **A remote, personal, private-network or local-filesystem module path:**
  rejected because it makes deployment topology part of the product API.
- **A public Go package to satisfy toolchain expectations:** rejected because a
  public package needs its own decision and an organization-owned module path.

## Consequences and Boundary

The repository remains portable across GitLab, GitHub, local checkouts, and
future hosting changes. Consumers install released products rather than import
unstable internal packages.

## Evidence and Revisit

Revisit if AIGW deliberately publishes a supported Go library API with an
organization-owned module namespace and compatibility contract.
