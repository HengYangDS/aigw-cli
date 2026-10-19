## 1. Inventory

- [x] 1.1 Calculate candidates with the pinned Renovate image (`dryRun=full`
      on GitHub `dev`) and list the stale direct inputs (October 19; 14 of 55).
- [x] 1.2 Check each candidate's identity, advisories and release notes (npm
      and OSV advisories empty; Mise options recognized).

## 2. Refresh

- [x] 2.1 Advance the Mise, npm, Go and CUE declarations and regenerate their
      native locks and peer projections (`6032e0d5`).
- [x] 2.2 Resolve the Mise lock twice with a byte-identical second result, and
      bootstrap the updated graph (`dependencies:resolve`).
- [x] 2.3 Repair the tests that the new golangci-lint reports.

## 3. Acceptance

- [x] 3.1 Pass strict OpenSpec, dependency-policy, lint and format gates and the
      touched tests on the lane head.
