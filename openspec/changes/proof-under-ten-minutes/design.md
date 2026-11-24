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

### The display order takes a complete recorded list as it is

- **Rests on:** `Config.AppendModel` sets the recorded order to `append(c.ModelRefs(), ref)`; `ModelRefs` sorted every
  model; decoding a configuration appends every model, so decoding n models sorted n times. In the profile it was 21 %
  of the matrix test's CPU.
- **Considered:** caching `ModelRefs`; changing `AppendModel`; making the shared reader `inDisplayOrder` cheap.
- **Chosen because:** every list (`ProviderIDs`, `FamilyIDs`, `ModelRefs`) goes through `inDisplayOrder`, so one change
  there serves all three, and `AppendModel` stays as it was. It rests on one invariant: no recorded list names an
  identity twice. Each writer records a list that these methods returned, that list with one identity it does not
  hold, or (`MergeTeamFile`) one such list followed by another's entries that it does not hold. A recorded list as
  long as the entries that names none that is gone is then exactly the entries, and is returned as it is; otherwise
  only the entries that it does not name are sorted. `ModelRefs` groups by provider rank only when the list is not
  grouped already, which is what a stable sort gives.
- **Evidence:** a reference test of the old sorting order over 400 random sequences of appends, direct writes and
  removals, which also requires that no recorded list repeats an identity; the fresh review compared the package at
  `51fdc384f` and at `374d29588` over 3,000 sequences of 60 steps, with families, `Clone`, `Normalize` and 12,825 team
  merges, and found the same four lists throughout.
- **Applies to:** `internal/domain/ordering.go`.

### A stored configuration is decoded, not compared

- **Rests on:** `Synchronizer.stored` read the stored configuration through the view of
  `Snapshot.MatchesConfiguration` and dropped the comparison, which encoded two configurations for nothing.
- **Chosen because:** `Snapshot.Configuration` decodes it, and `MatchesConfiguration` uses it too. One case differs: a
  working configuration that does not encode (it fails validation) made `stored` report false before; it now returns
  what is stored, as its contract says. Its callers pass a loaded configuration, and a selection's configuration is
  validated again by `PrepareSelection`.
- **Applies to:** `internal/store/store.go`, `internal/app/synchronization/selection.go`.

### Acceptance tests are parallel unless they change process state

- **Rests on:** 8 of 427 tests call `t.Parallel`; each test builds its own App over its own temporary folders; three
  tests change the whole process (two `t.Setenv`, one `t.Chdir`), which Go refuses in a parallel test, and Go runs
  serial top-level tests before parallel ones, so they never overlap.
- **Considered:** splitting the package; adding `t.Parallel` by hand and relying on review; a test of the package that
  parses its own files; golangci-lint's `paralleltest`, which the pinned golangci-lint ships.
- **Chosen because:** `t.Parallel` keeps one package and one coverage run. `paralleltest`, scoped to the package and to
  top-level tests, refuses a test without `t.Parallel` and passes the `t.Setenv` and `t.Chdir` tests (measured with
  probes, and on the whole module with a probe outside the package, which it passes), so a parsing test of our own
  would repeat it. A test that cannot run in parallel for another reason says why in a `nolint:paralleltest`
  directive. Neither notices a test that changes the process by other means (`os.Setenv`, `os.Chdir`, the umask);
  none does today. The race detector in `go-behavior` reports shared state that parallel runs expose.
- **Applies to:** `internal/cli/acceptance`, `.config/checks/golangci-lint/golangci-lint.toml`.

### A program that a test starts is written under the fork lock

- **Rests on:** the fresh review: tests write a program and then start it (106 uses of `executableFixture`, 21 of the
  keychain worker); with the tests in parallel, another test's child can hold the write descriptor until it starts its
  own program, and Linux refuses to start a file open for writing (ETXTBSY, Go issue 22315). Two tests also wrote a
  text file that the product starts as Codex, the input on which hosted Windows ended a test binary with 0xc0000409.
- **Considered:** retrying in the product (wrong owner); writing every program before the tests start; holding
  `syscall.ForkLock`, which every fork of the process holds for writing, while the file is open.
- **Chosen because:** the lock removes the window at its cause and needs no change to any test's paths; the text files
  become the silent program.
- **Applies to:** `internal/cli/acceptance/fixtures_test.go`, `keychain_worker_test.go`, `sync_test.go`,
  `client_test.go`.

## Risks / Trade-offs

- Parallel tests can expose shared state that serial runs hid; the race detector and the hosted platforms are the
  check, and a test that cannot run in parallel says why in its `nolint` directive. The race runs measured so far are
  on macOS: at two workers (320.9 s) and at full width inside `mise run check` (150.1 s); Linux and Windows are
  measured only by the proposal's hosted checks.
- The native credential operation's 5-second bound (`internal/secret/native`) now meets tests running beside the
  keychain tests; on the hosted Windows runner, where a new program's start is slow, that may fail. The hosted checks
  decide; the product's bound is not widened for a test.
- `tools/ci/main_test.go` lists every gate's arguments, so the selection-matrix gate's pattern is written there and in
  `tools/ci/quality.go`; this predates the Change and is not changed by it.
- The proof's time depends on the host's load; the acceptance measure names the load it was taken at.
- More parallel tests put more CPU on the host inside the same ticket; the ticket already admits a full proof as
  heavy work.
