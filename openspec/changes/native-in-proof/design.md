## Context

The proposal gives the repetition. Today the quality graph's `coverage` gate (`tools/ci/quality.go`, source only) runs
`go run ./tools/coverage --race --profile-output build/verification/coverage/profile.out`; the exact-HEAD proof's
`go-behavior` gate runs that command with a JUnit and an LCOV report (`.ethos/profile.toml`, held to it by
`requireGoAttempt` in `tools/ci/ethos_profile_test.go`). `tools/ci native` runs `check-go` and then
`nativeCommands`' test command: the same coverage command with `--tags=native_resource_acceptance` and a JUnit report,
inside a temporary workspace given by `construction.NativeTestEnvironment` (its own `TMPDIR`, forge credentials
cleared, performance attribution off), and on Linux under `dbus-run-session` with the Secret Service variables. The tag
adds one file, `tools/release/verification_resources_test.go`. The hosted quality jobs run `tools/ci quality`, which
leaves the source-only coverage gate out; the hosted native jobs run `tools/ci native`.

## Goals / Non-Goals

**Goals:** one Go test attempt per local verification of a head, which includes the host's native resource tests; one
owner of that attempt's command and environment.

**Non-Goals:** those of the proposal.

## Decisions

### D1. One command owns the host's native test attempt

- **Rests on:** the attempt needs a temporary workspace created before and removed after it, so a fixed command line
  in the quality graph or the ETHOS profile cannot carry it; the coverage gate and the proof gate both execute a fixed
  command line.
- **Considered:** (a) add the tag to the coverage gate's command alone, leaving the workspace and the Linux session
  out; (b) a mode of `tools/ci` that runs the test command of `nativeCommands` in its environment, with optional
  reports; (c) make the proof run `tools/ci native`, whose `check-go` the static gate already runs.
- **Chosen because:** (b) keeps the environment and the Linux wrapper with the code that already owns them and runs
  nothing twice; (a) would run the native resource tests against the real `TMPDIR` and forge credentials, and (c)
  repeats `check-go`.
- **Applies to:** `go run ./tools/ci native-tests [--junit-output <path>] [--lcov-output <path>]`, the `coverage`
  gate, the `go-behavior` gate, and `nativeCommands`, which builds its test command with the same function.

### D2. What a local verification now runs

`tools/ci source` and the proof run the native attempt once each, as the coverage attempt of that run; `tools/ci native`
without options runs `check-go` and the same attempt, so on this host it adds nothing that the proof does not run, and
the operator's route stops running it separately. The hosted native jobs keep their options and platforms.

The quality graph's `native-resource-fixture` gate runs `TestVerificationResourceCleanupStopsOwnedFixture` alone, in
the static gate's `tools/ci quality`; once the attempt carries the tag, the proof would run that test twice, which the
repository refuses for the selection matrix on the same ground. Both hosted native jobs run the attempt, and the
quality job runs on Linux beside the Linux native job, so no platform loses the test when the gate goes.

## Risks / Trade-offs

- The proof's behavior gate grows by the native resource tests (building one release candidate and its client cases)
  → the separate native run, 4 min 56 s in its own ticket on 2026-11-28, goes; the proof of this Change measures the
  new gate.
- A hosted quality job no longer runs the fixture test by itself → the native job of its platform runs it in the same
  pipeline, and an admission needs both jobs.
- A proof host without a usable native credential store fails the behavior gate → that host already fails
  `mise run native`, which the repository's required verification names.

## Migration Plan

None for the product. Rollback is a revert of the Change.
