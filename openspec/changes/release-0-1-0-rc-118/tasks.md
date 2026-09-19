## 1. Release source

- [x] 1.1 Advance `VERSION` to `0.1.0-rc.118` and verify release-source validation passes.
- [x] 1.2 Move the accepted dependency changes from `[Unreleased]` into the dated `0.1.0-rc.118` section and verify changelog chronology.

## 2. Release-source proof

- [ ] 2.1 Pass strict OpenSpec validation and the complete source gate.
- [ ] 2.2 Pass native macOS release acceptance from the exact release source.

Hosted CI, exact-ref publication, tag publication, Releases, native Linux and
Windows acceptance, installation, rollback, uninstall, and residue cleanup are
post-archive lifecycle effects governed by the release policy.
