## 1. Rule

- [ ] 1.1 Set the two zero-age package rules of the dependency policy to one day and add
      `TestRenovateHoldsEveryReleaseForADay`; the test fails on the old policy and passes on the new one, and
      `mise run dependencies:check` accepts the policy.
- [ ] 1.2 State the one-day floor and the registry without release times in the `supply-chain` requirement and in
      the dependency section of the change and release policy; strict OpenSpec validation of the Change passes.

## 2. Supply under the rule

- [ ] 2.1 Remove the re2js override that markdownlint 0.42.1 made unnecessary; the lock still resolves re2js to
      the release markdownlint requires, and the npm audit of the lock stays clean.
- [ ] 2.2 Run the candidate calculation with the new policy and record its result: it admits `go.yaml.in/yaml`
      v4.0.0 (a major with a new import path) and Claude Code 2.1.322 (it moves with Codex in one Windows supplier
      archive), and holds Codex 0.169.0, jsdom 31.0.1, Prettier 3.10.1 and both tool images by age. Each needs its
      own admission in the refresh of 2026-11-14 to 2026-11-16; the npm lock refresh waits for OpenSpec 2.0.0 to
      pass three days at 2026-11-14T22:46Z.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 3.2 The hosted graph passes at the proposal head on both peers.
