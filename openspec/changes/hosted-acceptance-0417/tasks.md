## 1. Mise Declaration

- [x] 1.1 Pin Mise 2026.10.19 in the CUE model, with its official image and
      Windows ARM64 digests, and raise the repository floor to the same release.
      Regenerate both Forge projections. The mirror failure test fails with the
      2026.10.15 binary and passes with the declared one.

## 2. Coverage

- [x] 2.1 Add regression tests for the uncovered 0.4.17 retry, rollback and
      release-peer paths. They pass, and the targeted statements are covered.

## 3. Verification

- [x] 3.1 `mise run check` passes on the lane head (`49666c25`, statement
      coverage 95.17%).
- [x] 3.2 The full proof of the lane head passes (Attestation `79ca7057`).
