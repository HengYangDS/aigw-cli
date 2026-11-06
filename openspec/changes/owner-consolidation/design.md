## Context

The review behind `adapter-simplification` also traced copies outside the
client adapters, in `internal/platform` consumers, the store, the updater and
the repository tools. `internal/platform` and `internal/platform/process` sit
below every package that repeats a step, so each owner can live there or in
the package whose callers it serves. Test fixtures copied between packages stay
where they are: `docs/architecture/repository-tooling.md` keeps test readers
package-local.

## Goals / Non-Goals

**Goals:**

- One owner for each step that several packages or tools repeat.
- No hook, flag or toggle that only tests set.
- A preview that refuses what its write refuses.

**Non-Goals:**

- Any other change to released behavior, including file bytes, modes, write
  and rollback order, error text, CLI output and exit codes.
- A shared test-support package.

## Decisions

### Owners sit where every consumer may import them

The SHA-256 helper and the capture-and-check runner go to `internal/platform`
and `internal/platform/process`, which adapters, the updater and every tool may
import. A helper in `tools/release/construction` could not serve
`tools/coverage`, which sits in the same layer.

### Repository tools gain the cleanup their runners lacked

`runAttached` in `tools/ci` and the runner in `tools/coverage` start processes
with a bare `exec.Command`, so they skip the process-group and Job cleanup that
the other runners have. Moving them onto `process.Runner.RunStream` adds that
cleanup. The tools are not released programs, so the change stays inside the
repository.

### Coverage profiles use the standard parser

`golang.org/x/tools/cover` already parses the mode line, merges duplicate blocks
and refuses an inconsistent statement count, and `go.mod` already requires it
indirectly. The coverage tool keeps its mode check and per-package sums.

### Previews run the write's transition check

`Synchronizer.Plan` skips the `validateTransition` that `PreviewProjection` and
`CommitProjection` run. `sync` and `repair` previews move to
`invocation.PreviewProjection`, which `client` and `uninstall` already use, so
a preview and its write refuse the same transitions. An accepted preview keeps
its output.

## Risks / Trade-offs

- A preview of a transition that the write refuses now fails instead of
  printing a plan. That plan was never applicable, and the error names the
  reason the write gives.
- The store's component table must keep every file name, message, mode and
  order. Its existing tests compare them, and each step stays one loop.
