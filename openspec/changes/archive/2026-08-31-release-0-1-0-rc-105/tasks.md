## 1. Release source

- [x] 1.1 Advance `VERSION` to `0.1.0-rc.105` and verify the release-source
      validator accepts it.
- [x] 1.2 Move the accepted onboarding correction from `[Unreleased]` into the
      dated `0.1.0-rc.105` section and verify changelog chronology.

## 2. Release-source proof

- [x] 2.1 Pass strict OpenSpec validation and the complete source gate.
- [x] 2.2 Pass native macOS release acceptance from the release source.

Hosted CI, exact-ref publication, tag publication, Release assets, native Linux
and Windows acceptance, installation, and residue cleanup are post-archive
lifecycle effects governed by the release policy. They are not editable Change
tasks.
