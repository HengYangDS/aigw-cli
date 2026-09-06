## 1. Locked Toolchain

- [x] 1.1 Change the source-command contract to the v4 executable name and verify the focused test fails against the old production command.
- [x] 1.2 Update `go.mod` and `mise.toml`, regenerate `mise.lock`, and verify a clean locked install resolves Go 1.27.1 and EditorConfig Checker 4.0.1.
- [x] 1.3 Regenerate CI projections from `.config/ci/pipeline.cue` and verify the projection tests pass without hand-edited drift.

## 2. Acceptance and Deletion

- [x] 2.1 Run strict OpenSpec validation, the focused CI tests, the real EditorConfig check, and the complete source and native release gates.
- [x] 2.2 Verify the final repository diff has one EditorConfig backend and executable contract, with no alias, wrapper, or parallel implementation.
- [x] 2.3 Record exact-ref landing, both SSH remote checks, superseded host-runtime removal, and Work Lane retirement as post-archive closeout operations that require live evidence.
