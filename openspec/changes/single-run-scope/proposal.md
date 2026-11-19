## Why

The archived Change `single-test-run` states that no other proof gate runs the Go tests. That is not true:
`go-static-analysis` runs `tools/ci quality`, which runs four focused `go test` selections under acceptance
build tags. What the Change removed is the second run of the coverage attempt's tests. A review of the lane
also found that, without `--json`, the coverage command hid the output of a test that never reports a result,
such as a test that times out or that a panic interrupts, and that two tests left room for a regression:
the Node test files were declared three times with nothing tying them, and the reports test did not count
test invocations.

## What Changes

- The requirement and its proof scenario say that no other proof gate runs that attempt's tests a second
  time; the change policy, the proof profile's comment and the changelog say the same.
- Without `--json`, the coverage command shows the output of a failed package's tests that reported no
  result, and of a passed test whose output arrived after its result.
- A proof-profile test requires `node-behavior` and the quality graph's `node-source` to run the same Node
  test files, and those to be the tracked `*.test.mjs` files; it derives the Go gate's command from the
  quality graph's coverage gate instead of repeating it. The proof-profile tests move to a file of their
  own.
- The reports test counts one `go test` invocation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `quantitative-quality`: the one-attempt requirement states its exact scope and shows the output of a
  test that reports no result.

## Impact

`openspec/specs/quantitative-quality/spec.md` through this delta, `tools/coverage` (the event recorder and
its tests), `tools/ci` (the proof-profile tests), `.ethos/profile.toml` (a comment),
`docs/governance/change-and-release-policy.md` and `CHANGELOG.md`.

- **Authority boundary:** unchanged from `single-test-run`; the archived Change stays as it was recorded.
- **Non-goals:** the four tagged `go test` selections in `tools/ci quality`, which test other build
  configurations and stay where they are.
