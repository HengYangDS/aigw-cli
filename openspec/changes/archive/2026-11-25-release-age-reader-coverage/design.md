## Context

`node --test --experimental-test-coverage` collects the coverage of the Node processes that a test starts, so
`tools/ci/test/text.test.mjs` claims `tools/ci/format.mjs` and `tools/ci/markdown/lint.mjs`, which it runs as child
processes (the proof's artifact of 2026-11-25 12:18 lists both as the gate's selected paths). Go's coverage cannot claim a
Node file.

## Goals / Non-Goals

**Goals:**

- The proof's behavior obligation holds for every source file, the reader included.

**Non-Goals:**

- A change of what the reader checks, or of the fill and check in Go.

## Decisions

### The reader's tests move to Node

- **Rests on:** the proof's gap above and the coverage of the existing Node test.
- **Considered:** keeping the Go test and adding a Node one (two tests of the same cases); claiming the file in the Go
  gate (Go cannot measure it).
- **Chosen because:** one test of each case, in the gate that can measure the file; the Go fill tests keep their
  accepting runner, and the quality graph's `release-age` gate still runs the real reader on the real files.
- **Applies to:** `tools/ci/test/release-age.test.mjs`, `tools/ci/release_age_test.go`.

### Both Node test commands name the new file

- **Rests on:** the proof gate `node-behavior` in `.ethos/profile.toml` and the quality graph's `node-source` gate in
  `tools/ci/quality.go` each list the Node test files.
- **Chosen because:** the local proof and both Forges must run the same tests; the two lists already exist for their two
  consumers.
- **Applies to:** `.ethos/profile.toml`, `tools/ci/quality.go` and the tests that list the quality commands.

## Risks / Trade-offs

- Adding a Node test file touches four places: the two commands and the two tests that pin them by value
  (`tools/ci/main_test.go`, `tools/ci/quality_test.go`). `requireNodeAttempt` in `tools/ci/ethos_profile_test.go`
  already fails while the two commands and the tracked `*.test.mjs` files differ, so the literal pin in
  `quality_test.go` repeats that guard; removing it is left to a later change, since this one moves a test only.
