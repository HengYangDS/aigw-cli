## 1. The judgement

- [x] 1.1 Write the failing tests: the twelfth review's findings 1 and 4 for Codex and for Hermes, each with the
      Token and without it, from its probes `zz12_review_targets_test.go`, `zz12_review_respelled_after_test.go` and
      `zz12_review_withdrawn_test.go`. Done when each fails on the lane's base for the reason the review gives.
- [x] 1.2 Give the judgement one owner in `internal/platform` and make `admitTargets`, `withdrawnClients`, Codex's
      `reconciliationInputs` and `codexTargetUnion`, and `hermesPlans` decide kept and dropped files by it. Done when
      1.1's tests pass, the existing suites pass, and no decision about a client's targets compares their paths as
      strings.
- [x] 1.3 Make Hermes write its file through a symbolic link that names it, so that the link stays a link and its
      target holds what AIGW writes, using the one canonical path form. Done when a test that names Hermes's file by a
      link fails on the lane's base and passes after, with and without the Token.
- [x] 1.4 Answer the fresh review of `84f6fa5eb`: Codex's catalog path compared by the one judgement, and a missing
      file whose existing folder is named in another case judged as one file; a target named by a hard link stays
      out of scope, as the design states. Done when the review's probes for the first two pass and the hosted Windows
      acceptance, which stopped the test binary of `84f6fa5eb` (exit status 0xc0000409), passes.
- [x] 1.5 Make every native acceptance run keep a JUnit report beside its coverage profile, so a test binary that
      the platform ends without output (hosted Windows' 0xc0000409 on this lane) still names the tests that never
      finished; then fix what it names: the fixture for a client's executable becomes a program (design: "A client's
      executable in the acceptance tests is a program"). Done when hosted Windows names its last test and then passes.

## 2. Verification

- [x] 2.1 Compile and vet the changed packages and their tests for Windows and Linux on the lane head.
- [x] 2.2 A fresh review of the lane's whole change, by a reviewer who has not seen it, against this Change's
      requirement and the twelfth review's two findings. Done when its findings are answered in code or in this design.
- [x] 2.3 Pass `mise run check` on the lane head.
- [x] 2.4 Pass the proposal's hosted checks on both forges.
