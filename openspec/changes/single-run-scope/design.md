## Context

`single-test-run` made the coverage attempt the one run of the Go test suite in the exact-HEAD proof. Its
wording overreached, and a review found a gap in the coverage command's plain output. This Change corrects
both. Its decisions follow the measured `go test -json` streams below.

## Goals / Non-Goals

**Goals:** accurate scope in the requirement and the documents; no hidden output of a failing test; tests
that tie each declaration to its owner.

**Non-Goals:** moving or removing the tagged `go test` selections of the quality graph; changing the
archived `single-test-run` record.

## Decisions

### What the proof no longer repeats

`go-static-analysis` runs `tools/ci quality`, whose graph holds four `go test` selections under the
`client_acceptance`, `native_resource_acceptance`, `performance_acceptance` and `dependency_acceptance` tags.
Those tests build only under their tags, so the coverage attempt does not run them. The accurate statement
is that no other proof gate runs the coverage attempt's tests a second time.

### Which test output the plain view shows

On 2026-11-20, Go 1.27.4's `go test -json` produced these streams in a throwaway module:

- **Timeout** (`-timeout=2s`, a test that sleeps): the test's output, the timeout panic included, belongs to
  that test, which never reports a result; the package then fails.
- **Panic during another test** (a goroutine started by a passed test panics while the next test runs): the
  panic belongs to the running test, which never reports a result; the package then fails.
- **Panic after every test passed** (`TestMain` waits after `m.Run`): the panic belongs to the package
  itself, after its `PASS` line; the package then fails.
- **Data race** (`-race`): the race report belongs to the racing test, which then fails.

In none of them did output arrive for a test after that test's result. The recorder had shown a test's
output only when that test failed, so the first two shapes left a person with `FAIL <package>` and nothing
else, though the JUnit report held the output. When a package fails, the recorder now shows the output of
every test that reported no result, and of every passed test that received output after its result (a
shape not seen, kept so that no output is lost). The JUnit report adds that late output to the package's
failure case.

### Tests tie declarations instead of repeating them

The Node test file appeared in the proof profile, in the quality graph and literally in the profile test,
and nothing required a new `*.test.mjs` to run anywhere. The test now requires both gates' test files to
equal the tracked `*.test.mjs` set. It derives the Go gate's command from the quality graph's coverage gate
plus the two report flags. The proof-profile tests move to `tools/ci/ethos_profile_test.go`, as
`mise_tasks_test.go` stood at 497 of 500 code lines.

## Risks / Trade-offs

- Output of a test without a result appears after the package's `FAIL` line, as the event that tells the
  recorder the package failed arrives last.
