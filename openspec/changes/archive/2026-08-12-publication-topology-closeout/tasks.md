# Tasks

- [x] 1.1 Replace the retired flat remote declaration with two explicit peers.
- [x] 1.2 Bind local verification and installation to repository-owned commands.
- [x] 2.1 Validate the complete change and execute exact-HEAD proof.
- [x] 3.1 Land the change and re-evaluate independent remote publication.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-control-plane:Local-first independent publication topology` | `1.1` | `ethos plan --changed --json` |
| `product-control-plane:Local-first independent publication topology` | `1.2` | `mise exec --locked -- go run ./tools/ci source` |
| `product-control-plane:Local-first independent publication topology` | `2.1` | `ethos prove --execute --expect-head <HEAD> --json` |
| `product-control-plane:Local-first independent publication topology` | `3.1` | `ethos land --json; ethos publish --probe-remote --expect-head <HEAD> --json` |
