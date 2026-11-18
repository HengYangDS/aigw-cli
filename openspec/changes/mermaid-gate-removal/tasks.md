## 1. Removal

- [x] 1.1 Remove the `check-mermaid` gate, its adapter and its tests, and its entries in the quality graph.
      Done when `go test ./tools/ci` and `node --test tools/ci/test/text.test.mjs` pass and the graph still
      supplies every required concern of each carrier class.
- [x] 1.2 Remove `@mermaid-lint/core` and its jsdom override from `package.json`, regenerate the lock, and
      drop the dependency policy's license corrections for packages that no longer resolve. Done when the
      lock holds 107 packages, no picomatch 2.x, KaTeX 0.19.1, and `npm audit signatures` passes.
- [x] 1.3 The text and change policies stop naming a diagram validator, and `CHANGELOG.md` records the removal.

## 2. Verification

- [ ] 2.1 `mise run check` passes on the lane head with this Change and `yaml-v4-import-path`.
