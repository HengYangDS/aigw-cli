# Tasks

## 1. Reproduce and repair

- [x] 1.1 Observe the projected target revision regression fail for the
      intended reason.
- [x] 1.2 Add the minimal cross-platform Git helper condition.

## 2. Verify

- [x] 2.1 Run focused and complete `tools/forge` tests.
- [x] 2.2 Run the complete repository coverage gate without changing its
      policy.

## Delivery Boundary

Exact-HEAD proof, archive, candidate integration, accepted closeout, Forge
projection, hosted CI, release, and lane retirement remain lifecycle effects
proven separately.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-control-plane:Native cross-platform release admission` | `1.1` | `focused-red-target-revision` |
| `product-control-plane:Native cross-platform release admission` | `1.2` | `focused-green-target-revision` |
| `product-control-plane:Native cross-platform release admission` | `2.1` | `tools-forge-package-pass` |
| `product-control-plane:Native cross-platform release admission` | `2.2` | `coverage-tools-forge-96.05-aggregate-96.55` |
