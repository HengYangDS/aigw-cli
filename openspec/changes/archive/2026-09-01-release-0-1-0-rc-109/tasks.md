## 1. Release source

- [x] 1.1 Advance `VERSION` to `0.1.0-rc.109` and verify
      `go run ./tools/release validate-release-sources` passes.
- [x] 1.2 Move the accepted credential-observation correction from
      `[Unreleased]` into the dated `0.1.0-rc.109` section of `CHANGELOG.md`
      and verify changelog chronology.

## 2. Release-source proof

- [x] 2.1 Pass strict OpenSpec validation and the complete source gate.
- [x] 2.2 Pass native macOS release acceptance from the exact release source.

Hosted CI, exact-ref publication, tag publication, Release assets, native Linux
and Windows acceptance, installation, rollback, uninstall, and residue cleanup
are post-archive lifecycle effects governed by the release policy. They are not
editable Change tasks.
