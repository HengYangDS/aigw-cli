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

- The two lists of Node test files must stay equal; this Change adds one name to each and does not merge them.
