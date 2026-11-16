## 1. Supply

- [x] 1.1 Advance Lip Gloss to v2.1.1; `go mod tidy` changes nothing else, and the packages that render with
      it pass their tests.
- [x] 1.2 Advance the Renovate image to 44.242.0, pinned to its index digest; `mise run dependencies:check`
      accepts the policy.
- [x] 1.3 Advance `jdx/mise-action` to v5.2.9 by its commit in the CUE authority; the GitHub workflows, the
      one projection that names it, follow, and `tools/ci project --check` reports no drift.
- [x] 1.4 Refresh the npm lock within its declared ranges (three transitive packages move, none is added or
      removed); registry signatures verify, `npm audit` reports no advisory, and the formatter changes no
      file.

## 2. Release

- [x] 2.1 Prepare 0.8.1: `VERSION`, and the changelog heading dated by the UTC day.

## 3. Verification

- [x] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [x] 3.2 The hosted graph passes at the proposal head on both forges.
