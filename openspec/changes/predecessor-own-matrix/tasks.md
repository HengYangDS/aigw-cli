## 1. Published matrix

- [x] 1.1 Add the published matrix to `internal/program/artifact`: the names that a signed manifest lists, bound to
      its version, and their validation. Done when its tests pass, including a predecessor matrix with archives the
      current matrix lacks.
- [x] 1.2 Verify a predecessor in `tools/release/provenance` by its published matrix and rebuild its provenance with
      the manifest's subjects. Done when the tests of both pass.
- [x] 1.3 Use them in `preparePublishedBaseline` and take the predecessor's files of the native input package as its
      manifest names them. Done when the construction tests pass.

## 2. Verification

- [ ] 2.1 A fresh review of this Change by a reviewer who has not seen it. Done when its findings are answered in the
      code or in this design.
- [ ] 2.2 Pass `mise run check` on the lane head.
- [ ] 2.3 Pass the proposal's hosted checks on both Forges.
