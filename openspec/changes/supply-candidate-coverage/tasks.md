## 1. Candidate rules

- [x] 1.1 Replace the Renovate rule for package-name client strings with one
      rule for each native client pin of the CUE authority, Codex and Claude
      Code from npm and Hermes from its GitHub tags; a candidate calculation
      on the lane head reads all three with their pinned values.
- [x] 1.2 Add a dependency test that fails unless exactly one candidate rule
      reads each native client pin with its pinned value; it fails on the
      previous policy and passes on the new one.

## 2. Refresh

- [ ] 2.1 Advance uv to 0.13.7 with `mise lock` entries for all six
      platforms.
- [ ] 2.2 Advance the Hermes pin to v0.22.6, its tag commit and the SHA-256 of
      that release's `scripts/install.sh`; the projection test follows and
      `tools/ci project --check` reports no drift.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head.
- [ ] 3.2 A candidate calculation on the lane head reports no update except
      Claude Code 2.1.317, held by its release age.
- [ ] 3.3 The hosted graph passes at the landed head, and the native client
      journeys on macOS, Linux and Windows pass with Hermes v0.22.6.
