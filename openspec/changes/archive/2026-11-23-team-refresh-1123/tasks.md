## 1. Team file

- [x] 1.1 Apply the proposal's entries and removals to `manifests/team.toml`, with the families that enter and leave.
      Done when the team-file tests pass and `TestTeamManifestKeepsOneFamilyPerCuratedLine` holds the four Gemini 4
      lines.
- [x] 1.2 Record the qualification in `docs/research/provider-model-qualification.md` and the change in
      `CHANGELOG.md`. Done when the text and link checks pass.

## 2. Verification

- [x] 2.1 `mise run check` passes on the lane head.
- [x] 2.2 The proposal's hosted checks pass on both forges (GitHub at `eb1084853`: 5 passed, 2 skipped; GitLab pipeline
      17334; again after `dev` was merged, at `02c1ae272`: GitHub 5 passed, 2 skipped; GitLab pipeline 17368).
