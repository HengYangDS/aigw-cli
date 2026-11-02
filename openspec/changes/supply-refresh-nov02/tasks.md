## 1. Refresh

- [x] 1.1 Advance the Mise image to 2026.10.25 with its Windows ARM64
      executable and shim digests and the Mise floor, the Renovate image to
      44.199.2, mise-action to v5.2.5, `go.yaml.in/yaml/v3` to v3.0.6 and the
      Codex pin to 0.166.0; after regenerating both Forge projections and the
      retained-step digest, `tools/ci project --check` reports no drift.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head, and the full proof of the lane head passes.
- [ ] 2.2 The hosted graph and the native client journeys on macOS, Linux and
      Windows pass at the lane head.
