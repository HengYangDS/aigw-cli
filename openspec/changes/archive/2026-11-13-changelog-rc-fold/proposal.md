## Why

The history rebuild planned for about 2026-11-22 retags only the 35 tags that have an ETHOS release record and retires
the other 51 `v0.1.0-rc.*` tags; the owner decided this on 2026-11-10. Once those tags are gone, the 51 CHANGELOG
sections named after them would be released sections without a tag, which the release gate refuses, so ETHOS's proof
would fail before the rebuild's `base`. The fold must land first.

The `release-distribution` requirement also says that the CHANGELOG has one-to-one parity with local product tags.
The validator has never enforced the direction from tags to sections, which `TestChangelogAllowsUnlistedLocalTag…`
pins: v0.3.2 and v0.4.0 are tags without sections today. The requirement should state the rule the gate applies.

## What Changes

- Fold the 51 sections `0.1.0-rc.58` to `0.1.0-rc.118` into the `0.1.0` section, category by category, keeping
  every item's text and its newest-first order. All 230 items remain; the file loses 328 lines.
- Restate the `release-distribution` requirement: every released section has its tag, a tag may lack a section, and
  a release carries its candidates' changes.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `release-distribution`: a release section carries its pre-release candidates' changes, and the gate requires a tag
  for every released section rather than a section for every tag.

## Impact

`CHANGELOG.md` and the specification change. The validator and its tests already apply the restated rule. The
program and the released artifacts do not change.
