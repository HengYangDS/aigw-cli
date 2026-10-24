# Tasks

## 1. Rooted Windows Catalog Reference

- [x] 1.1 Make Windows tests expect a rooted user catalog reference to take the
      drive of the configuration's directory, a drive-relative reference to
      stay unchanged and a plain relative reference to join that directory, and
      make the projection-preservation test name a reference that is absolute
      on every platform. Hosted Windows acceptance of `f2194fe2` failed that
      test for this reason.
- [x] 1.2 Resolve non-absolute references through platform files; the macOS
      tests pass, and the Windows tests compile and pass `GOOS=windows` vet.

## 2. Verification

- [ ] 2.1 `mise run check` passes, and `GOOS=windows` lint passes.
- [ ] 2.2 The full proof of the lane head passes.
