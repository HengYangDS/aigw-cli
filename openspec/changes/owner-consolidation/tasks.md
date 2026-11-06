## 1. Shared helpers and repository tooling

- [ ] 1.1 Keep one SHA-256 helper in `internal/platform` and use it from every
      streaming and byte-digest copy; digests and their formats are unchanged.
- [ ] 1.2 Keep the capture-and-check process runner once in
      `internal/platform/process`; `tools/ci` and release construction use it,
      and attached runners stay in the caller's process group.
- [ ] 1.3 Give `tools/ci` one slash-relative file list, one scrubbed
      `git ls-files` invocation and one scratch-directory helper; every check's
      findings are unchanged.
- [ ] 1.4 Parse coverage profiles with `golang.org/x/tools/cover`; the coverage
      tool reports the same per-package numbers and total.
- [ ] 1.5 Give release publication one admission, one asset comparison and one
      GitLab configuration builder; refusals and messages are unchanged.
- [ ] 1.6 Run the architecture checks directly from the root's policy file,
      with no flag, table or toggle that only tests set.
- [ ] 1.7 Bound the `mise` children of the tools/ci tests by one constant that
      hosted Windows meets.

## 2. Product code

- [ ] 2.1 Hold the store's four optional components in one table; file names,
      bytes, modes, order and messages are unchanged.
- [ ] 2.2 Serve the updater's authorized GETs, asset downloads and release
      resolution from one helper each; error text is unchanged.
- [ ] 2.3 Remove `doctor`'s test-only inspector.
- [ ] 2.4 Make `sync --dry-run` and `repair --dry-run` refuse what `sync` and
      `repair` refuse; a regression test per command proves it, and CHANGELOG's
      `[Unreleased]` section records it.

## 3. Verification

- [ ] 3.1 Run the local quality graph at the lane head.
- [ ] 3.2 Confirm the hosted three-platform graph on the proposal.
