# Design

## Decisions

1. Preserve one Account/Profile/Route control-plane model and the existing
   Codex and Claude Code adapters.
2. Fix hosted failures at their semantic owners: repository-controlled
   fixtures and focused package tests.
3. Treat README commands and public environment names as product contracts,
   not explanatory prose.
4. Keep Python tooling out of the Go repository environment.
5. Update only dependencies selected by the Go module graph; do not add a new
   package or compatibility layer to force unrelated transitive upgrades.

## Verification

- native Go tests, static analysis, portability, governance, and coverage;
- README presentation and text-layout checks;
- hosted Linux, Windows, and macOS jobs before release.
