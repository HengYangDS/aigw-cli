## Context

See `proposal.md`. The repository already owns Renovate through one digest-pinned OCI reference in `mise.toml`.

## Goals / Non-Goals

**Goals:** advance that existing owner to the current stable immutable image and reuse the established validation graph.

**Non-Goals:** change dependency policy, introduce automation, modify product behavior, or refresh unrelated dependencies.

## Decisions

- Replace only the existing tag and index digest.
- Use the registry's multi-platform index digest so Linux AMD64 and ARM64 remain bound to one release identity.
- Reuse existing dependency and source gates; no new script or state carrier is justified.

## Risks / Trade-offs

- A newly published upstream image may be withdrawn or malformed. Mitigation: validate the pinned image offline through the existing dependency gate before integration.
