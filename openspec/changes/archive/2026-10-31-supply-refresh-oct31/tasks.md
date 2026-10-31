## 1. Refresh

- [x] 1.1 Advance the Mise image to 2026.10.24 with its Windows ARM64
      executable and shim digests and the Mise floor, the Renovate image to
      44.198.3, and the native client pins to Codex 0.165.0 and Claude Code
      2.1.312; after regenerating both Forge projections and the retained-step
      digest, `tools/ci project --check` reports no drift.
- [x] 1.2 Move real-client acceptance, the provider catalog and route evidence,
      and repository tooling into documents of their own, so that every
      authored Markdown document holds at most 500 nonblank lines; the Markdown
      and link checks pass.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head, and the full proof of the lane head passes.
- [x] 2.2 The hosted graph and the native client journeys on macOS, Linux and
      Windows pass at the lane head.
