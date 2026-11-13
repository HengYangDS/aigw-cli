## 1. Team file

- [x] 1.1 State the line rule in the `progressive-team-onboarding` delta; strict OpenSpec validation of the Change
      passes.
- [x] 1.2 Add GPT-6 Astra, GPT-6 Luna, Gemini 4 Hydrogen and UCloud's Claude Fable 5.2 with `tool_search` to
      `manifests/team.toml`, each provider model with three passing Hermes turns on the stock Hermes that CI pins.
- [x] 1.3 Add `TestTeamManifestKeepsOneFamilyPerCuratedLine` and pin the new models in the team-file and Desktop
      tool-search tests; the line test fails on the file of 0.7.1 and on a file that holds two Astra families.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane head (`38815de1`, statement coverage
      95.55%).
- [x] 2.2 The hosted graph passes at the proposal head (`38815de1`: GitLab MR !193 pipeline 15336, GitHub PR 259
      run 43390603848).
