# Tasks

- [x] 1.1 Confirm accumulated product changes are archived and the lane is clean.
- [x] 1.2 Bind the complete lane delta and candidate compare-and-swap permission.
- [x] 1.3 Execute exact-HEAD full proof.
- [ ] 1.4 Archive the integration Change and land by exact candidate CAS.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-control-plane:Terminal candidate integration is exact and local` | `1.3` | `ethos prove --execute --full --expect-head <HEAD> --json` |
