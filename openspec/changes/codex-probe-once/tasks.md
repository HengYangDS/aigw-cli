## 1. Specification

- [x] 1.1 Write the delta of `client-programs`. Done when `openspec validate codex-probe-once --strict` passes
      and the lane's archive rehearsal merges it.

## 2. One probe per unchanged program

- [x] 2.1 Make the seam `codexBundledCatalog` a reader that keeps one probe per key of D1 for the process, failures
      included, shares it between concurrent reads and returns a copy of the table (D1 to D5). Done when
      `TestAnUnchangedProgramIsProbedOncePerProcess`, `TestAChangedProgramIsProbedAgain`, `TestAFailedProbeIsKept`,
      `TestAnUnreadableProgramIsNotKept`, `TestConcurrentReadsOfOneProgramShareOneProbe`,
      `TestReadsOfDifferentProgramsDoNotWait`, `TestAProbeOfOtherBytesIsNotKept`,
      `TestAReadThatWaitedForAnUnfinishedProbeFails` and `TestTheReturnedTableIsACopy` pass.
- [x] 2.2 Pin that a sync at the adapter layer starts an unchanged Codex once for its version and once for its table.
      Done when `TestASyncStartsAnUnchangedCodexOnce` and the package tests of `internal/adapter/codex` pass.

## 3. Documents

- [x] 3.1 Add the change under `## [Unreleased]` in `CHANGELOG.md`. Done when
      `go run ./tools/release validate-changelog` passes.

## 4. Measurement

- [ ] 4.1 Measure the selection matrix (`-tags=selection_matrix ./internal/cli/acceptance`) on the lane head in the
      host queue, as the proposal's 608 s were measured. Done when its wall time, system time and peak memory are
      recorded beside the baseline in the lane's evidence.

## 5. Local gates

- [ ] 5.1 Pass `mise run check` on the lane head, in the host queue.
- [ ] 5.2 Pass `mise run native` on the macOS host on the lane head, in the host queue.

## 6. Review

- [x] 6.1 A fresh review of the whole change by a reviewer who has not seen it. Done when its findings are answered in
      code or in the design. Eight findings, no defect of correctness (`build/ethos/claude-210a1fe7/review.md` in the
      lane), answered in code and in D1 to D5; its speed finding waits for 4.1.

## 7. Exact-HEAD proof

- [ ] 7.1 Pass the ETHOS full proof of the lane head.

## 8. Hosted CI

- [ ] 8.1 Pass the proposal's GitHub checks `Native macOS acceptance`, `Native Linux acceptance`,
      `Linux Secret Service` and `Quality and governance`.
- [ ] 8.2 Pass the proposal's GitLab pipeline on the macOS and Linux runners.

## 9. Publication

- [ ] 9.1 Publish the proven lane head as the lane's proposal branch on both forges. Done when each forge reads back
      the exact head.
