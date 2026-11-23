## 1. The judgement

- [x] 1.1 Write the failing tests: the twelfth review's findings 2 and 3, each with the file credential store and a
      store that cannot tell, from its probes `zz12_review_filestore_test.go`, `zz12_review_keychain_test.go` and
      `zz12_review_tokens_test.go`, and finding 5 for Codex and Hermes. Done when each fails on the lane's base for the
      reason the review gives.
- [x] 1.2 Give the judgement three answers in `internal/secret` and make `ConvergeBinding`, Hermes's plans,
      synchronization and `aigw client edit` decide by it, treating a Token that cannot be observed as an absent one
      for the clients that use it. Done when 1.1's tests for findings 2 and 3 pass and the existing suites pass.
- [x] 1.3 Withdraw AIGW's settings and record from each file that `aigw client edit --target` drops without the
      Token, instead of refusing the edit. Done when 1.1's tests for finding 5 pass.
- [x] 1.4 State the changed behaviour in `CHANGELOG.md` and in the command documentation of `aigw client edit`. Done
      when the text and link checks pass.

## 2. Verification

- [x] 2.1 Compile and vet the changed packages and their tests for Windows and Linux on the lane head.
- [x] 2.2 A fresh review of the lane's whole change, by a reviewer who has not seen it, against this Change's
      requirements and the twelfth review's findings 2, 3 and 5. Done when its findings are answered in code or in
      this design.
- [ ] 2.3 Pass `mise run check` on the lane head.
- [ ] 2.4 Pass the proposal's hosted checks on both forges.
