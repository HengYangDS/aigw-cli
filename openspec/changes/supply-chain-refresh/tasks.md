## 1. Dependency Graph

- [x] 1.1 Refresh the Go module graph and run `go mod tidy`; verify every direct module is current and unused historical checksums are removed.
- [x] 1.2 Confirm mise and npm direct tools remain current; verify `mise outdated --json` and `npm outdated --json` are empty.

## 2. Verification and Closeout

- [x] 2.1 Run the complete source gate and native release gate against the refreshed graph.
- [x] 2.2 Confirm the final diff contains only the OpenSpec record and removal of unused module checksums; verify `git diff --check` passes before commit.
