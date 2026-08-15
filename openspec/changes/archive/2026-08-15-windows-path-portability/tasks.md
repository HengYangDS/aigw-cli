## 1. Make projection paths host-independent

- [x] 1.1 Add failing contracts for repository path conversion and CUE process binding.
- [x] 1.2 Validate repository paths with slash semantics and reject host-specific input.
- [x] 1.3 Run CUE from the repository root with a relative model path.

## 2. Verify and deliver

- [x] 2.1 Pass focused and complete `tools/ci` tests in the locked toolchain.
- [x] 2.2 Pass deterministic projection drift verification.
- [x] 2.3 Pass complete repository source verification in the locked toolchain.

## Delivery Boundary

Archive, candidate integration, accepted closeout, dual-Forge publication, CI,
release, and lane retirement remain lifecycle effects proven separately.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:portable repository text` | `1.1` | Focused RED failed because the portable command contract did not exist. |
| `product-quality:portable repository text` | `1.2` | Focused path tests pass. |
| `product-quality:portable repository text` | `1.3` | Focused command test and render tests pass. |
| `product-quality:portable repository text` | `2.1` | Complete `tools/ci` package tests pass. |
| `product-quality:portable repository text` | `2.2` | `ci project --check` passes without projection drift. |
| `product-quality:portable repository text` | `2.3` | `tools/ci source` passes with statement and branch coverage above 95%. |
