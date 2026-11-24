## 1. Each test once, and less work per test

- [x] 1.1 Give the selection matrix's tagged tests a prefix of their own and select only it in the `selection-matrix`
      gate. Done when a test of the quality graph finds that the gate's pattern matches no test of the acceptance
      package's untagged files, and the tagged run lists only tagged tests.
- [ ] 1.2 Make `Config.AppendModel` append to a recorded order that already holds every model, without sorting. Done
      when a test shows the same order as `ModelRefs` gave before for configurations decoded from files, built by
      appends, and mixed with models set into the map directly, and a profile of the matrix test no longer shows
      `ModelRefs` among its top costs.
- [ ] 1.3 Profile the matrix test again and fix any further product hot spot it names, each with its own test. Done
      when the profile's top product costs are the commands' own file reads and writes.

## 2. Parallel acceptance tests

- [ ] 2.1 Call `t.Parallel` first in every top-level test of the acceptance package that does not call `t.Setenv`.
      Done when the package passes under the race detector with coverage, as `go-behavior` runs it.
- [ ] 2.2 Add a test of the package that reads its own test files and requires `t.Parallel` first in every top-level
      test unless the test calls `t.Setenv`. Done when it fails on a new test without `t.Parallel` and passes on the
      package.

## 3. Verification

- [ ] 3.1 Pass `mise run check` on the lane head.
- [ ] 3.2 A fresh review of the lane's whole change by a reviewer who has not seen it. Done when its findings are
      answered in code or in this design.
- [ ] 3.3 A full proof of the lane head held 600 s or less on this host, with its admission load recorded. Done when
      the proof record shows it; if it does not, the remaining cost and the matrix's place in the proof go to the owner
      as a decision.
- [ ] 3.4 Pass the proposal's hosted checks on both forges.
