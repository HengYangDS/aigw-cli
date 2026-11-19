## Context

The exact-HEAD proof has two behaviour gates. `go-behavior` runs the coverage command, which runs the Go
tests once with the race detector and enforces the coverage policy. `code-behavior` uses ETHOS's behaviour
provider without native outputs, and that provider runs `go test -json -coverpkg=./... -count=1 ./...` again
to see which files executed, then runs the Node tests of the two `.mjs` adapters under its own Node custody.
The provider gate depends on the coverage gate, so the second Go run adds its whole duration to every proof.

ETHOS 0.2.0-beta.10 also accepts a command gate that lists the behaviour provider under
`verification_providers` and declares `native_outputs`. ETHOS then runs the command once in an owned output
directory, binds the reports to the command, head, source tree and exit status of that run, and judges them
without running another tool (`ethos/adapters/gates/native/evidence.py`, read on 2026-11-19). The judgment is
language-neutral: it needs a `junit-xml` report and an `lcov` or `cobertura-xml` report. Its rules are:

- every JUnit report parses, has at least one test case and no failure or error, and its declared suite
  totals match its cases;
- every coverage record names a tracked file;
- an applicable source file that a coverage report measures must have at least one executed line;
- a file that a coverage report measures with no lines counts as exercised, and an applicable file that no
  report names is unclaimed;
- the union of the files that the behaviour gates claim must equal the proof's behaviour subjects.

## Goals / Non-Goals

**Goals:** one Go test run per proof; the same behaviour obligation over the same files; the coverage policy
and its single executor unchanged.

**Non-Goals:** the repository quality graph, CI jobs and `mise run check`; the Node tests that the quality
graph runs inside `go-static-analysis`; any change to ETHOS.

## Decisions

### The coverage command writes both reports from its one attempt

With `--junit-output`, the command adds `-json` to its one `go test` invocation and reads the event stream as
it arrives. Without `--json`, stdout shows what a plain run shows: package lines and the output of failing
tests. With `--json`, the event bytes pass to stdout unchanged, as before. The command writes one JUnit test
suite per package that ran tests and one test case per test result, subtests included, with failures and
skips. A package that fails without a failing test, as a build failure or a panic outside a test does, gets
one test case that carries the error, so the report cannot read as a pass. A test that starts and never
reports a result is an error.

With `--lcov-output`, the command lists the selected packages' files with `go list -json` under the same tags.
It maps each statement-profile file to its repository path and writes one LCOV record per production file,
sorted by path:

- **Compiled, with statements:** each line that a statement block spans, with the highest count among the
  blocks on that line. Blocks without statements add no line.
- **Compiled, without counters:** an empty record. Its package is observed, so the build instrumented no
  statement in it, which is what ETHOS's own Go check reports as not applicable.
- **Excluded from this platform's build** (a non-test `IgnoredGoFiles` entry): an empty record, as ETHOS's own
  Go check claims these files without coverage.

A profile file outside the listed inventory fails the command. LCOV carries line records only; it states no
branch or function figure, and the statement threshold stays with the coverage policy, which the command
still enforces on the same profile. Both reports are written through a temporary file in the target
directory and renamed, so a reader never sees a partial file. They are written after the test attempt even
when it fails, as diagnostic evidence; the command then still fails.

On 2026-11-19, `go list` on `fbec6f90d` gave 298 tracked production Go files: 279 compiled and 19 excluded on
macOS (13 Windows, 2 Linux, 4 other platform variants). The legacy proof of `517b4a412` reported 26 files not
applicable: those 19 and 7 compiled files without statements.

Alternatives considered:

- **Keep ETHOS's second run.** Rejected: it costs about 100 s per proof and proves the same tests.
- **`gotestsum`, `go-junit-report` v2, `gcov2lcov`, `gocover-cobertura`.** Each would add a dependency or a tool to
  the lock, through the dependency policy's age admission and audit, to produce two formats. The mapping
  needs `encoding/xml` and the profile parser that the command already uses.
- **Cobertura instead of LCOV.** Either is accepted. LCOV is line-oriented and needs no package or class
  hierarchy, so it maps directly to the profile's file blocks.

### Node tests keep their own small gate

The quality graph's `node-source` gate already runs `node --test tools/ci/test/text.test.mjs` inside
`go-static-analysis`. Making that gate `SourceOnly` would remove the Node tests from CI's quality job, which
runs `tools/ci quality` and not `source`, so it stays. A `node-behavior` proof gate runs the same test file
with `--experimental-test-coverage` and Node's `junit` and `lcov` reporters. It has no dependency, so it runs
beside the Go gate. On 2026-11-19 this command took 9.8 s on the lane. ETHOS's parsers read its reports as
54 passing test cases, and its coverage as `tools/ci/format.mjs` (51 of 53 lines) and
`tools/ci/markdown/lint.mjs` (32 of 34 lines). It runs through `mise exec --locked`, so ETHOS applies no
Node custody of its own.

## Risks / Trade-offs

- **ETHOS no longer recomputes Go coverage.** It checks the reports' structure, outcomes, per-file execution
  and custody, but a defect in the command's report writer would go unseen by ETHOS. Mitigation: the writer
  has unit tests for each classification and outcome, and the coverage command still enforces the statement
  threshold and package observation on the same profile.
- **LCOV lines are derived from statement blocks.** A block's span can include comment lines, so ETHOS's
  informational line percentage differs from the statement percentage. No threshold reads it.
- **The Node tests still run twice per proof** (in `go-static-analysis` and in `node-behavior`), as they do
  today. The two runs take about 9 s together and run beside the Go gate.

## Migration Plan

The gates change in one commit with the command. Rollback restores the previous `.ethos/profile.toml` and the
structure test. The new flags are optional, so other callers of the coverage command are unaffected.
