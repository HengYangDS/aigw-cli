# Tasks

## 1. Authority and inventory

- [x] 1.1 Record exact heads, leases, worktrees, dirty overlays, and independent Forge state.
- [x] 1.2 Classify every historical lane as absorbed, uniquely useful, or discardable.
- [x] 1.3 Rebuild only unique product semantics in this terminal lane; never merge an old tree wholesale.

## 2. Product correctness

- [x] 2.1 Prove independent Codex and Claude Code discovery, planning, and atomic projection.
- [x] 2.2 Prove absent clients are untouched and future adapters do not change current clients.
- [x] 2.3 Prove ordinary providers are declarative and optional diagnostics remain isolated.
- [x] 2.4 Remove compatibility shells, forwarding facades, hard-coded host identity, paths, and Forge coupling.

## 3. Product and repository quality

- [x] 3.1 Converge semantic packages, UX/DX surfaces, docs, decisions, and configuration SSOTs.
- [x] 3.2 Refresh the latest stable locked supply chain without duplicated CI pins.
- [x] 3.3 Prove the repository-owned formatting, vetting, static analysis, security, links, architecture, release, and native-platform contracts locally.
- [x] 3.4 Prove statement, branch, package, and aggregate coverage are each strictly above 95%.

## Delivery boundary

Archiving, exact-HEAD proof, branch-role transitions, independent Forge
publication, formal installation, runtime acceptance, and lane retirement are
post-Change lifecycle effects. Their public command receipts and the active
delivery Goal own those facts; duplicating them here would make Change
completion depend on its own archive operation.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-control-plane:Independently admitted native clients` | `2.1` | `internal/codex; internal/claude` |
| `product-control-plane:Declarative ordinary provider extension` | `2.3` | `internal/configuration; internal/providers` |
| `product-control-plane:Portable source and user contract` | `2.4` | `tools/ci source; internal/cli/acceptance` |
| `product-quality:one complete quality graph` | `3.1`, `3.3` | `tools/ci source; hosted native jobs` |
| `product-quality:faithful quantitative quality evidence` | `3.4` | `tools/ci source` |
| `product-control-plane:Latest stable repository-owned supply chain` | `3.2` | `mise exec --locked -- go run ./tools/ci source` |
| `repository-organization:Portable repository quality surface` | `3.1` | `tools/ci source` |
| `repository-organization:Governed release-branch convergence` | `3.3` | `tools/forge contracts; public lifecycle receipts after archive` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `3.3` | `tools/forge contracts; hosted pipelines` |
