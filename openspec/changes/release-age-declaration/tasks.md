## 1. The declaration and the fill

- [x] 1.1 Declare the age in `[settings]` of `.config/mise/config.toml` as `minimum_release_age` of 24h. Done when `mise settings get minimum_release_age` in the lane reads 24h from that file and `TestMiseConfigurationKeepsToolResolutionRepositoryBound` passes.
- [x] 1.2 Add to `tools/ci` the function from hours to Renovate's line and npm's line (zero leaves npm's out) and `tools/ci/release-age.mjs`, which takes the declared hours and Renovate's policy on stdin and names each age that differs from it or belongs to another rule. Done when unit tests follow a declaration of 0, 1, 24, 25 and 72 hours, refuse a declaration that is not whole hours before any write, and leave every other line as it was.
- [x] 1.3 Add `ci release-age` (fill) and `ci check-release-age` (check), register the check as the quality gate `release-age` for the `dependency-policy` and `toolchain` carrier classes, and make `mise run fix` run the fill before the format step. Done when the quality graph and command-list tests pass, the check names an edited npm line, a second rule of its own is refused, and `mise run fix` restores the line.
- [x] 1.4 Fill the two settings from the declaration with `mise run fix`: Renovate's root age and npm's `min-release-age`. Done when `ci release-age` passes, `dependencies:check` passes and `TestMiseNpmEnvironmentIsBoundToThisRepository` passes with the new npm line.

## 2. Documents

- [ ] 2.1 Move the dependency maintenance section of `docs/governance/change-and-release-policy.md` to `docs/governance/dependency-maintenance.md`, because the policy stands at its 500-line ceiling; say there where the age is declared and that `mise run fix` fills Renovate's and npm's settings from it, and point `CONTRIBUTING.md` and `docs/governance/README.md` at it. Done when the readiness proof reports no document-length gap and the documentation tests pass. The changelog keeps user-visible changes of the product, so it gets no entry.

## 3. Verification

- [x] 3.1 Run `npm ci --ignore-scripts` in a scratch copy of the lane under the filled npm line and state the result in the design. Done when the design holds the measured outcome.
- [ ] 3.2 Pass `mise run check` on the lane head.
- [ ] 3.3 A fresh review of the lane's whole change by a reviewer who has not seen it. Done when its findings are answered in code or in this design.
- [ ] 3.4 Pass the proposal's hosted checks on both Forges.
