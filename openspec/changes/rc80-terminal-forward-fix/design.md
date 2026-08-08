# Design

## Decisions

1. Preserve one Account/Profile/Route control-plane model and the existing
   Codex and Claude Code adapters.
2. Fix hosted failures at their semantic owners: repository-controlled
   fixtures and focused package tests.
3. Treat README commands and public environment names as product contracts,
   not explanatory prose.
4. Keep repository governance and release semantics in Go or portable POSIX
   shell; delete Python-only parallel owners rather than translating them into
   another wrapper layer.
5. Update only dependencies selected by the Go module graph; do not add a new
   package or compatibility layer to force unrelated transitive upgrades.
6. Use one Decision Record register and the
   `dr-<sequence>-<description>.md` grammar for durable rationale; OpenSpec
   remains the only change authority.
7. Keep `config.toml` as the sole cross-platform control-plane file name. A
   cosmetic internal rename cannot create a second path, compatibility reader,
   or migration burden at a user data boundary.

## Verification

- native Go tests, static analysis, portability, governance, and coverage;
- README presentation and text-layout checks owned by the Go architecture gate;
- hosted Linux, Windows, and macOS jobs before release.
## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-control-plane:Portable source and user contract` | `13.1` | `ethos-exact-head-proof` |
