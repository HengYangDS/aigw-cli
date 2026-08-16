## 1. Make the failure proof portable

- [x] 1.1 Add a failing focused regression that requests a deterministic directory-read error.
- [x] 1.2 Route the production directory read through one private operation seam.
- [x] 1.3 Remove the POSIX permission fixture and retain the same error assertion.

## 2. Verify and deliver

- [x] 2.1 Pass the focused architecture package tests in the locked toolchain.
- [x] 2.2 Pass the complete repository verification graph before exact-HEAD execution.

## Delivery Boundary

Archive, candidate integration, accepted closeout, independent Forge
publication, release assets, runtime installation, and lane retirement remain
separate lifecycle effects with their own evidence.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:portable repository text` | `1.1` | `focused-red` |
| `product-quality:portable repository text` | `1.2` | `focused-architecture-tests` |
| `product-quality:portable repository text` | `1.3` | `portable-fixture-review` |
| `product-quality:portable repository text` | `2.1` | `locked-architecture-package-tests` |
| `product-quality:portable repository text` | `2.2` | `complete-repository-verification` |
