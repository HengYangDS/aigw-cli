## Why

A fresh-context review on 2026-11-05 traced steps that more than one owner
holds, in product code and in repository tooling:

- Eight streaming SHA-256 copies and four byte-digest copies.
- Five process runners in the tools, two of which skip process-group and Job
  cleanup.
- `tools/ci` converts repository paths back and forth, lists tracked files
  twice and creates its scratch directory three times.
- `tools/coverage` hand-parses the Go cover profiles that
  `golang.org/x/tools/cover` parses.
- Release publication repeats its admission, asset comparison and GitLab
  configuration.
- `tools/architecture` keeps knobs that only tests turn.
- The store spells out its four optional components one by one.
- The updater repeats its authorized GET, its asset download and its release
  resolution.
- `doctor` keeps an inspection hook that only tests set.
- Two `tools/ci` tests bound a `mise` child at a fixed 20 seconds, which a
  hosted Windows runner exceeded on PR 245.

The same review found that `sync --dry-run` and `repair --dry-run` build their
plan without the transition check that `sync` and `repair` run, so a preview can
show a plan that the write then refuses.

## What Changes

- Each repeated step gets one owner that every consumer may import: SHA-256 in
  `internal/platform`, the capture-and-check runner in
  `internal/platform/process`, one file list and scratch directory in
  `tools/ci`, `golang.org/x/tools/cover` for coverage profiles, one admission
  and asset comparison in release publication, one component table in the
  store, and one authorized GET, download and release resolver in the updater.
- Hooks that only tests set go: `doctor`'s inspector and the architecture
  scanner's injection table, flag and toggles.
- `sync --dry-run` and `repair --dry-run` refuse a transition that `sync` and
  `repair` refuse, with the same error.

Non-goals: no other released behavior, persisted file, CLI output or
configuration format changes, and no shared test-support API is introduced.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: a step that product packages and repository tools
  repeat has one owner where every consumer may import it.
- `command-line`: a preview refuses what its write refuses.

## Impact

`internal/platform`, `internal/store`, `internal/program`, `internal/cli`
(`doctor`, `sync`, `repair`) and `tools` (`ci`, `coverage`, `release`,
`architecture`) change. `golang.org/x/tools` becomes a direct requirement at the
version `go.mod` already names. The preview fix is visible to operators and goes
into CHANGELOG's `[Unreleased]` section; no release follows from this Change
alone.
