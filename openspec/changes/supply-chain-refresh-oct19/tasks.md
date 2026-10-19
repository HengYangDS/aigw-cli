## 1. Inventory

- [x] 1.1 Calculate candidates with the pinned Renovate image (`dryRun=full`
      on GitHub `dev`) and list the stale direct inputs (October 19; 14 of 55).
- [x] 1.2 Check each candidate's identity, advisories and release notes (npm
      and OSV advisories empty; Mise options recognized).

## 2. Refresh

- [ ] 2.1 Advance the Mise, npm, Go and CUE declarations and regenerate their
      native locks and peer projections.
- [ ] 2.2 Resolve the Mise lock twice with a byte-identical second result, and
      bootstrap the updated graph.
- [ ] 2.3 Repair the tests that the new golangci-lint reports.

## 3. Acceptance

- [ ] 3.1 Pass strict OpenSpec, dependency-policy and source gates and the full
      ETHOS proof of the lane head.
- [ ] 3.2 Replace the Windows ARM64 runner's Mise with the pinned release in a
      window without jobs, and verify its digests and version on the runner.
- [ ] 3.3 Integrate the signed result and verify local and both Forge `dev` and
      `main` refs and their CI.
