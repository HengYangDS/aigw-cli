# Tasks

## 1. Authority and inventory

- [x] 1.1 Record exact heads, leases, worktrees, dirty overlays, and independent Forge state.
- [ ] 1.2 Classify every historical lane as absorbed, uniquely useful, or discardable.
- [ ] 1.3 Rebuild only unique product semantics in this terminal lane; never merge an old tree wholesale.

## 2. Product correctness

- [x] 2.1 Prove independent Codex and Claude Code discovery, planning, and atomic projection.
- [x] 2.2 Prove absent clients are untouched and future adapters do not change current clients.
- [x] 2.3 Prove ordinary providers are declarative and optional diagnostics remain isolated.
- [ ] 2.4 Remove compatibility shells, forwarding facades, hard-coded host identity, paths, and Forge coupling.

## 3. Product and repository quality

- [ ] 3.1 Converge semantic packages, UX/DX surfaces, docs, decisions, and configuration SSOTs.
- [ ] 3.2 Refresh the latest stable locked supply chain without duplicated CI pins.
- [x] 3.3 Prove formatting, vetting, static analysis, security, links, architecture, release, and native platforms.
- [x] 3.4 Prove statement, branch, package, and aggregate coverage are each strictly above 95%.

## 4. Delivery and acceptance

- [ ] 4.1 Archive the completed change, execute full proof, and land the exact revision.
- [ ] 4.2 Close `candidate/dev`, accepted `dev`, and release `main` through public governance commands.
- [ ] 4.3 Publish matching signed assets independently to GitLab and GitHub and verify each Forge separately.
- [ ] 4.4 Install the formal artifact and prove portable Codex and Claude Code configuration UX.

## 5. Housekeeping

- [ ] 5.1 Retire every absorbed or discarded worktree, branch, and lease with owner-bound evidence.
- [ ] 5.2 Remove temporary checkouts, caches, generated residue, and obsolete remote proposals.
- [ ] 5.3 Recheck canonical roots, protected branches, CI, releases, installation, and zero remaining next actions.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-control-plane:Independently admitted native clients` | `2.1` | `internal/codex; internal/claude` |
| `product-control-plane:Declarative ordinary provider extension` | `2.3` | `internal/configuration; internal/providers` |
| `product-control-plane:Portable source and user contract` | `2.4` | `tools/ci source; internal/cli/acceptance` |
| `product-quality:Coverage exceeds the product threshold` | `3.4` | `tools/ci source` |
| `product-quality:Quality is platform-complete` | `3.3` | `tools/ci source; hosted native jobs` |
| `product-quality:Supply-chain versions have one maintained authority` | `3.2` | `mise exec --locked -- go run ./tools/ci source` |
| `repository-organization:Portable repository quality surface` | `3.1` | `tools/ci source` |
| `repository-organization:Governed release-branch convergence` | `4.2` | `ETHOS exact-head proof and closeout receipts` |
| `ci-diagnostics:Hosted Git initialization is explicit` | `3.3` | `tools/forge contracts; hosted pipelines` |
