## 1. Construction

- [x] 1.1 Build the portable artifacts in a checkout that has no Git remote, with no diagnostic of GoReleaser:
      a test builds in such a checkout, names the scenario "No Forge is configured" and fails without the
      change.
- [x] 1.2 Build them as well in a detached checkout among several peers without `origin`, the one other state
      in which Git has no answer; a table row for each detached state and a build in that state fail without
      the change.

## 2. Verification

- [x] 2.1 `mise run check` passes on the lane head.
- [x] 2.2 The hosted graph passes at the proposal head on both forges.
