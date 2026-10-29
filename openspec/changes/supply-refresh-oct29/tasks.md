# Tasks

## 1. Refresh

- [x] 1.1 Advance Node to 26.13.0, uv to 0.13.6, gh to 2.103.3, glab to
      1.126.0, the Renovate image to 44.192.2 and the Mise floor to 2026.10.22
      in the Mise declaration, and regenerate their lock entries with
      `mise lock`; the lane's locked tools report each new version.
- [x] 1.2 Advance mise-action to v5.2.4 and the Mise image to 2026.10.22 with
      its Windows ARM64 digests, and pin Codex 0.164.0 and Claude Code 2.1.310
      in the CUE authority; after regenerating both Forge projections and the
      retained-step digests, `tools/ci project --check` reports no drift.
- [x] 1.3 Advance OpenSpec to 1.15.0 and the katex override to 0.19.1,
      refresh the npm lock within its declared ranges, and override
      markdownlint's re2js with 2.9.1; `npm audit` reports no vulnerability.

## 2. Verification

- [ ] 2.1 The dependency tasks, `mise run check` and the full proof of the lane
      head pass.
