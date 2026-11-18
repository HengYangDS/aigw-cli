## 1. Supply

- [x] 1.1 Advance npm, uv, gh, glab and Syft in the Mise configuration and regenerate its lock with `mise lock`.
      Done when `mise install --locked` installs each new release.
- [x] 1.2 Pin the Renovate image to 44.239.1, the newest release that was 72 hours old on 2026-11-18, by its index
      digest. Done when `mise run dependencies:check` accepts the policy.
- [x] 1.3 Advance OpenSpec to 2.1.0 in the npm lock. Done when registry signatures verify and every Change
      validates strictly.
- [x] 1.4 Advance `golang.org/x/tools` to v0.53.0. Done when `go mod tidy -diff` reports nothing and the coverage
      tool's tests pass.
- [x] 1.5 Advance the native clients to Codex 0.170.0, Claude Code 2.1.327 and Hermes v0.23.2 in the CUE
      authority. Done when `tools/ci project --check` reports no drift and the projection tests pass.

## 2. Verification

- [ ] 2.1 `mise run check` passes on the lane head.
