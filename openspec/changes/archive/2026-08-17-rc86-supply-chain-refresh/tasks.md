# Tasks

## Change

- [x] 1.1 Refresh the stable Go module graph and tidy the lock state.
- [x] 1.2 Set `VERSION` and `CHANGELOG.md` to `0.1.0-rc.86`.
- [x] 1.3 Run source, coverage, architecture, documentation, and release gates.
- [x] 1.4 Produce exact-HEAD proof and archive the completed Change.
- [x] 1.5 Land and close the accepted and release branches.

## Post-archive effects

- Publish independent GitLab and GitHub `v0.1.0-rc.86` releases.
- Install the signed portable release and run `aigw doctor --json`.
- Retire the owner lane and remove transient verification artifacts.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:Terminal local release readiness` | `1.1` | `go-mod-current-and-local-gates` |
