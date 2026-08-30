## Context

See [proposal.md](proposal.md). The repository already assigns Go dependency
authority exclusively to `go.mod` and `go.sum`; mise and npm own separate tool
closures and currently report no updates.

## Goals / Non-Goals

**Goals:**

- Update the existing Go graph in place.
- Preserve one dependency authority per ecosystem.
- Prove behavior, quality, native packaging, and Forge projection stability.

**Non-Goals:**

- A new dependency manager, compatibility graph, or product capability.
- Updating a dependency merely because a pseudo-version exists when the module
  resolver does not select it for the supported Go release.

## Decisions

Use the Go module resolver to refresh the complete graph, then retain only the
minimal `go.mod` and `go.sum` result after `go mod tidy`. This reuses the
language-native mechanism and introduces no repository-owned updater.

## Risks / Trade-offs

- **Risk:** a transitive update changes behavior. **Mitigation:** run the full
  source gate and native release gate before exact-HEAD proof.
- **Risk:** a listed upstream version is incompatible with the supported toolchain.
  **Mitigation:** accept only the graph resolved and verified by the locked Go
  toolchain; do not hard-code an override.
