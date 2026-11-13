## 1. Supply

- [x] 1.1 Advance the native client pins to Codex 0.169.0 and Claude Code 2.1.325 in the CUE authority; both
      Forge projections and the Windows client supplier name follow, and `tools/ci project --check` reports no
      drift.
- [x] 1.2 Advance Prettier to 3.10.1 and the jsdom override to 31.0.1, and refresh the npm lock within its
      declared ranges (24 transitive packages move, none is added or removed); registry signatures verify,
      `npm audit` reports no advisory, and the formatter changes no file.
- [x] 1.3 Advance the Renovate image to 44.239.1, pinned to its index digest; `mise run dependencies:check`
      accepts the policy.
- [x] 1.4 State in the `supply-chain` requirement that a refresh without a window keeps a runner-owned tool's
      pinned release and names the Change that moves it; strict OpenSpec validation of the Change passes.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane head (`80b0dd20`, statement coverage
      95.55%).
- [x] 2.2 The hosted graph passes at the proposal head on both peers (`80b0dd20`: GitLab MR !196 pipeline 15492,
      GitHub PR 262 run 43520957201), and the native client journeys on macOS, Linux and Windows pass with the
      advanced client pins (run 43514404696 at `6c66de70`, from v0.7.0; each job's log shows `codex-cli 0.169.0`
      and `2.1.325 (Claude Code)`).
