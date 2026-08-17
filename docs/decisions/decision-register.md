# Decision Records

Decision Records preserve durable product rationale after an OpenSpec change
closes. They do not duplicate specifications, task status, or incident history.

## File grammar

```text
dr-<four-digit-sequence>-<concise-kebab-case-description>.md
```

Sequences are stable and never reused. Records are amended or superseded, not
silently repurposed. Tool-mandated names such as `go.mod`, `spec.md`, and
OpenSpec carrier names keep their native grammar.

## Required sections

Every record contains Status, Date, Context, Decision, Consequences, and Revisit
Trigger. A record is required for a durable product boundary, foundational
architecture or dependency choice, compatibility or retention policy, release
trust, security posture, or another costly-to-reverse ruling.

## Register

| Record | Decision |
| --- | --- |
| [DR-0001](dr-0001-control-plane-data-plane-boundary.md) | Keep AIGW configuration control separate from transport data planes. |
| [DR-0002](dr-0002-github-release-provenance.md) | Accept signed, independently verified GitHub release provenance. |
| [DR-0003](dr-0003-account-rename-credential-migration.md) | Use two-phase account credential migration. |
| [DR-0004](dr-0004-account-profile-route-authority.md) | Use Account, Profile, and Route as the configuration authority. |
| [DR-0005](dr-0005-admitted-client-adapter-boundary.md) | Admit client adapters explicitly. |
| [DR-0006](dr-0006-transactional-client-projection.md) | Project all selected client state as one guarded transaction. |
| [DR-0007](dr-0007-local-first-independent-forge-release.md) | Keep local closure and independent Forge publication. |
| [DR-0008](dr-0008-non-fetchable-product-build-identity.md) | Use a non-fetchable product build identity. |
| [DR-0009](dr-0009-forward-only-current-product-semantics.md) | Keep only current product semantics. |
