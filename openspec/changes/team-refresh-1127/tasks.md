## 1. Team file

- [x] 1.1 Apply the proposal's entries and the replacement to `manifests/team.toml`, with the family that enters and
      the one that leaves. Done when the team-file tests pass.
- [x] 1.2 Record the qualification in `docs/research/provider-model-qualification.md` and the change in
      `CHANGELOG.md`. Done when the text and link checks pass.

## 2. Verification

- [x] 2.1 A fresh review of this Change by a reviewer who has not seen it. Done when its findings are answered in the
      files or in this design. Two reviews: the first answered in `849bbf8b6` with a third run, the second in the next
      commit with a fourth run.
- [x] 2.2 Pass `mise run check` on the lane head. Passed on `71caaddf` after the merge of dev, statement
      coverage 95.79%.
- [x] 2.3 Pass the proposal's hosted checks on both Forges. GitHub PR 290 and GitLab pipeline 18575 passed.
