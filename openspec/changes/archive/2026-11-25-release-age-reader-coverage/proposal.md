## Why

The archive proof of Change `release-age-declaration` (head `502c695dc`, 2026-11-25 12:18) stopped with
`quality_obligation_unproven:behavior`, although every gate passed. Its new Node reader, `tools/ci/release-age.mjs`, is
in neither behavior gate's selected paths: `go-behavior` lists it as unclaimed, and `node-behavior` runs only
`tools/ci/test/text.test.mjs`. The reader's tests are Go tests that start Node, so no coverage report of either gate
claims the reader.

## What Changes

- A Node test, `tools/ci/test/release-age.test.mjs`, runs the reader the way the existing Node test runs its modules,
  and the Go test that started the reader from Go is removed, so each of its seven cases is tested once. Three cases
  are new: quoted keys with a comment and a preset under `extends`, which reach the reader's two branches that the
  seven did not (13 of 13 with them, 11 without), and the legacy `stabilityDays`; the test also requires the verdict on
  stderr and nothing on stdout.
- The test joins the Node test command of the proof gate `node-behavior` and of the quality graph's `node-source` gate.

Out of scope: any change of the product, of the reader's behavior or of the specifications.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change moves a test so that the proof's behavior gate claims the reader, so it declares `skip_specs: true`.

## Impact

`tools/ci/test/release-age.test.mjs`, `tools/ci/release_age_test.go`, `.ethos/profile.toml`, `tools/ci/quality.go`,
and the tests that list the quality commands (`tools/ci/main_test.go`, `tools/ci/quality_test.go`).
