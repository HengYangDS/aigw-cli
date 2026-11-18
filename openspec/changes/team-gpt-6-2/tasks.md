## 1. Team file

- [x] 1.1 Replace GPT-6 Astra and GPT-6.1 Sol by GPT-6.2 Astra and GPT-6.2 Sol in `manifests/team.toml` on every
      provider that qualified, and move the Codex and Hermes recommendations to 6.2 Sol. Done when
      `TestTeamManifestKeepsOneFamilyPerCuratedLine` and the team-file tests pass.
- [x] 1.2 Record the qualification in `docs/research/provider-model-qualification.md` and the recommendation in
      the team rollout guide. Done when the text and link checks pass.
- [x] 1.3 State the owner's daily line and effort rule in the team rollout guide, with each layer's default. Done
      when the guide's table names Sol, Opus and Argon and the text checks pass.

## 2. Verification

- [ ] 2.1 `mise run check` passes on the lane head.
