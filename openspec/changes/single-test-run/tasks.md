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

- [ ] 3.1 Run `go-behavior` and `node-behavior` alone on the committed lane head under a host ticket. Done
      when ETHOS reports both gates passing and the behaviour obligation proved.
- [ ] 3.2 Full exact-HEAD proof of the lane head passes, with no Go test run outside `go-behavior`. Record its
      total and per-gate times beside the base proof of `fbec6f90d` (346 s; `go-behavior` 111 s,
      `code-behavior` 102 s).
