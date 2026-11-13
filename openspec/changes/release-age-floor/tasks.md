## 1. Rule

- [ ] 1.1 Declare the release age once at the root of the dependency policy, as `0 hours`, remove the package
      rules that only carried an age, and add `TestRenovateDeclaresTheReleaseAgeOnce`; the test fails for an age
      in days, for a rule with its own age and for a security-fix age above zero, passes for `24 hours`, and
      `mise run dependencies:check` accepts the policy.
- [ ] 1.2 State the declared release age and the registry without release times in the `supply-chain` requirement
      and in the dependency section of the change and release policy; strict OpenSpec validation of the Change
      passes.

## 2. Supply

- [ ] 2.1 Remove the re2js override that markdownlint 0.42.1 made unnecessary; the lock still resolves re2js to
      the release markdownlint requires, and the npm audit of the lock stays clean.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 3.2 The hosted graph passes at the proposal head on both peers.
