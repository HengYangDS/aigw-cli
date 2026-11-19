## Why

Every exact-HEAD proof runs the Go tests twice. The `go-behavior` gate runs them through the coverage
command with the race detector; the `code-behavior` gate then asks ETHOS's own behaviour check, which runs
`go test -json -coverpkg=./... ./...` again without it, and the second gate waits for the first. In the 33
full proofs that passed between 2026-11-06 and 2026-11-19 (this repository's ETHOS proof records), the
first gate took a median of 106 s and the second a median of 102 s (85 to 206 s); the proof of `fbec6f90d`
took 346 s, 102 s of it in the second run. On 2026-11-18 the repository owner asked for the utmost
performance in every ETHOS-governed repository, and on 2026-11-19 approved running the tests once.

ETHOS 0.2.0-beta.10 can judge a gate's own run: a command gate that declares a behaviour verifier and its
native outputs has its JUnit XML and LCOV or Cobertura reports checked, and ETHOS runs no test itself.
`go test` writes neither format, so the coverage command has to write them from its one attempt.

## What Changes

- The coverage command gains `--junit-output` and `--lcov-output`. With them it reads the same attempt's
  native test events and statement profile and writes a JUnit XML test report and an LCOV line report; the
  selected packages, tags, race option, instrumentation and threshold stay as they are.
- The `go-behavior` proof gate writes both reports into ETHOS's output directory and becomes the behaviour
  gate: ETHOS judges its reports instead of running the Go tests again. The `code-behavior` gate is removed.
- A `node-behavior` proof gate runs the Node tests of the two repository `.mjs` adapters with Node's own
  JUnit and LCOV reporters, so the behaviour obligation still covers them.
- The test that pins the proof gates' shape expects the new shape.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `quantitative-quality`: the requirement that one coverage attempt yields the test events and counters
  also yields the JUnit and LCOV reports, and the exact-HEAD proof reads that attempt instead of running the
  Go tests a second time.

## Impact

`.ethos/profile.toml`, `tools/coverage` (the command and its tests), `tools/ci/mise_tasks_test.go`,
`docs/governance/change-and-release-policy.md` and `CHANGELOG.md`.

- **Authority boundary:** the coverage policy keeps the statement threshold and package observation; the
  coverage command stays their one executor. ETHOS keeps the proof verdict and judges the reports' structure,
  outcomes, per-file execution and custody, but no longer recomputes Go coverage on its own.
- **Reuse:** ETHOS's native-output verifier, Node's built-in reporters and `golang.org/x/tools/cover`, which
  the command already uses; no new dependency.
- **Non-goals:** the repository quality graph, CI jobs and `mise run check`, which keep running the tests as
  they do now; the Node tests that the quality graph also runs inside `go-static-analysis`; the released
  program.
