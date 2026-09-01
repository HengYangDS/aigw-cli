## 1. Release source

- [x] 1.1 Advance `VERSION` to `0.1.0-rc.110` and verify the release-source
      contract.
- [x] 1.2 Record the accepted setup, Codex verification, and Linux credential
      changes in the dated `0.1.0-rc.110` section of `CHANGELOG.md`, then verify
      chronology.

## 2. Release-source proof

- [x] 2.1 Pass strict OpenSpec validation and the complete source gate.
- [x] 2.2 Pass native macOS release acceptance from the exact release source.

Hosted CI, tag and asset publication, native Linux and Windows acceptance,
installation, rollback, uninstall, and residue cleanup are post-archive
lifecycle effects governed by the release policy. They are not editable Change
tasks.
