## 1. Release source

- [x] 1.1 Advance `VERSION` to `0.1.0-rc.103` and verify the release metadata
      reads the same version.
- [x] 1.2 Move the accepted client-protocol health correction from
      `[Unreleased]` into the `0.1.0-rc.103` section of `CHANGELOG.md` and
      verify release chronology.

## 2. Release-source proof

- [x] 2.1 Pass strict OpenSpec validation and the complete locked source gate.
- [x] 2.2 Pass native macOS acceptance and build the reproducible artifact
      matrix without replacing the working rc.101 installation.
- [x] 2.3 Verify this completed Change is ready for the signed,
      behavior-neutral archive transition.

Hosted CI, exact-ref publication, tag publication, Release assets, native
Linux and Windows acceptance, installation, and residue cleanup are
post-archive lifecycle effects governed by the release policy. They are not
editable Change tasks.
