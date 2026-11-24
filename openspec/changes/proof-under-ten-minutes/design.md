## Context

A full proof of aigw-cli runs `go-static-analysis` (`./tools/ci quality`) first, the readiness tier's light gates
after it, then the test gates one at a time (`go-behavior`, then `node-behavior`), inside one host ticket. Since
2026-11-23 the static gate carries the exhaustive selection tests and the behavior gate a much larger acceptance
package; the figures are in the proposal.

## Goals / Non-Goals

**Goals:**

- A full proof of 600 s or less on this host, measured, with every test that runs today and the coverage floor kept.
- Each test runs once per proof.

**Non-Goals:**

- Dropping the exhaustive selection tests from the proof, or sampling them. They found the twelfth review's defects;
  they stay in every proof unless the measured result of this Change still misses the target, which would be put to the
  owner as a decision.
- Changing ETHOS's gate scheduling. Its test gates run one after another; moving the matrix to a gate of its own
  would not shorten the proof.

## Decisions

### The matrix gate runs only its own build tag's tests

- **Rests on:** the gate's `-run` pattern `^Test(EveryCommandLeavesEachRecordNamingWhatItsFileSelects|ZZ11)` matches 29
  untagged tests in `zz11_review_*_test.go`, 162 s of serial tests, which `go-behavior` also runs.
- **Considered:** narrowing the pattern by listing names; one prefix of the tagged file's own.
- **Chosen because:** a prefix that only the tagged file uses (`TestMatrix`) keeps the selection true as tests are
  added, and a test of the quality graph can check it against the package's untagged files.
- **Applies to:** `internal/cli/acceptance/selection_matrix_test.go`, `tools/ci/quality.go`.

### AppendModel appends to the recorded order

- **Rests on:** `Config.AppendModel` sets the recorded order to `append(c.ModelRefs(), ref)`; `ModelRefs` sorts every
  model; decoding a configuration appends every model, so decoding n models sorts n times. In the profile it is 21 %
  of the matrix test's CPU.
- **Considered:** caching `ModelRefs`; appending to the recorded order when it already holds every model.
- **Chosen because:** `ModelRefs` orders the recorded list by a stable sort of provider rank, so appending the new
  reference to a recorded list that already holds every model gives the same order as before, without a sort; a
  recorded list that does not hold every model (a model set into the map directly) keeps today's path.
- **Applies to:** `internal/domain/ordering.go`.

### Acceptance tests are parallel unless they change process state

- **Rests on:** 8 of 427 tests call `t.Parallel`; each test builds its own App over its own temporary folders; two tests
  call `t.Setenv`, which Go refuses in a parallel test, and Go runs serial tests before parallel ones, so they never
  overlap.
- **Considered:** splitting the package; adding `t.Parallel` by hand and relying on review.
- **Chosen because:** `t.Parallel` keeps one package and one coverage run; a test of the package that reads its own
  files and requires every top-level test to call `t.Parallel` first, unless it calls `t.Setenv`, keeps it so. The
  race detector in `go-behavior` reports any shared state that parallel runs expose.
- **Applies to:** `internal/cli/acceptance`.

## Risks / Trade-offs

- Parallel tests can expose shared state that serial runs hid; the race detector and the hosted platforms are the
  check, and a test that cannot run in parallel says why in the structure test's own list.
- The proof's time depends on the host's load; the acceptance measure names the load it was taken at.
- More parallel tests put more CPU on the host inside the same ticket; the ticket already admits a full proof as
  heavy work.
