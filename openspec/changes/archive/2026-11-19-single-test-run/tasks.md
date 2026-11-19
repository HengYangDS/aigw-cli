## 1. Coverage reports

- [x] 1.1 Add `--junit-output` to the coverage command: one `go test -json` attempt, events passed through
      unchanged under `--json`, the plain package view otherwise, and a JUnit report with failures, skips,
      package errors and unfinished tests. Done when focused tests of each outcome pass.
- [x] 1.2 Add `--lcov-output`: one record per selected production file, with lines for compiled files that
      have statements and empty records for compiled files without counters and for files excluded from this
      platform's build, and a refusal of a profile file outside the inventory. Done when focused tests of each
      classification pass and the package keeps its statement coverage.

## 2. Proof gates

- [x] 2.1 Make `go-behavior` write both reports into ETHOS's output directory as the behaviour gate, remove
      `code-behavior`, add the `node-behavior` gate, and update the structure test in
      `tools/ci/mise_tasks_test.go`. Done when `go test ./tools/ci ./tools/coverage` passes.
- [x] 2.2 Record the flags in the change policy and the change in `CHANGELOG.md`.

## 3. Verification

- [x] 3.1 ETHOS judges `go-behavior` and `node-behavior` from their own reports on the committed lane head.
      Done when both gates pass and the behaviour obligation is proved. Proof of `08c8dbcd6` under a host
      ticket: 6290 Go and 54 Node test cases, every behaviour subject claimed, same attempt.
- [x] 3.2 Full exact-HEAD proof of the lane head passes, with no Go test run outside `go-behavior`. Record its
      total and per-gate times beside the base proof of `fbec6f90d` (346 s; `go-behavior` 111 s,
      `code-behavior` 102 s). Proof of `08c8dbcd6`: 216 s; `go-behavior` 104 s, `node-behavior` 7 s after
      it; `go-static-analysis` 43 s against 80 s, a load difference outside this Change.
