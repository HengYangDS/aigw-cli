# DR-0009: Keep Only Current Product Semantics

- Status: accepted
- Date: 2026-08-07

## Context

Compatibility shims, alias packages, forwarding wrappers, dual readers, and
retired policy paths preserve historical implementation shapes inside the
current product. They enlarge the API and test surface while obscuring the
single semantic owner.

## Decision

AIGW source, tests, schemas, docs, commands, and tooling describe only the
current supported model. A breaking internal or repository-tooling change may
remove the old shape directly. Historical release and decision records remain
evidence, not executable compatibility surfaces.

Source-level shims, re-exports, alias-only packages, forwarding facades, and
silent legacy readers are forbidden unless a new current requirement explicitly
admits them with ownership and end-to-end proof.

## Consequences

The codebase remains lean and each behavior has one owner. Operators may need
an explicit documented migration at a true external data boundary rather than
an indefinite hidden compatibility layer.

## Revisit Trigger

Revisit only for a concrete external compatibility obligation whose users,
sunset condition, owner, and verification are explicitly defined.
