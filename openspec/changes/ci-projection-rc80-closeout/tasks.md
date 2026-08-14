# Tasks

- [x] 1.1 Define one declarative CI graph while keeping release targets, native evidence, and developer-host claims semantically distinct.
- [x] 1.2 Generate GitLab and GitHub files as deterministic projections.
- [x] 1.3 Fail source verification on projection drift or an invalid evidence graph.
- [x] 1.4 Delete the handwritten CI contract parser and duplicate policy.
- [x] 2.1 Run the complete local source and native macOS gates.
- [ ] 2.2 Execute exact-HEAD governance proof and integrate the owner lane.
- [ ] 3.1 Verify required GitLab and GitHub branch CI independently.
- [ ] 3.2 Create and verify independent signed `v0.1.0-rc.80` tags.
- [ ] 3.3 Verify immutable release assets, checksums, and release records on both Forges.
- [ ] 4.1 Retire absorbed CI lanes and remove their worktrees and refs through public lifecycle commands.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:one complete quality graph` | `1.1-1.4` | CUE validation, exact projection comparison, and Forge job DAG |
| `product-quality:one complete quality graph` | `2.1-2.2` | local source/native logs and exact-HEAD proof |
| `product-quality:complete delivery evidence` | `3.1-3.3` | independent CI, tag, release, asset, and checksum receipts |
| `product-quality:complete delivery evidence` | `4.1` | owner-bound lane retirement receipts |
