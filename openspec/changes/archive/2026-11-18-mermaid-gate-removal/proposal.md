## Why

The Mermaid gate validates nine diagrams; eight of them stand in archived Changes and one in current
documentation (`docs/concepts/product-concepts.md`). For that alone the npm lock holds 142 of its 249
packages, `@mermaid-lint/core` with Mermaid, jsdom and D3 among them. One of them, picomatch 2.3.2 through
micromatch, is named by GHSA-fc5q-39xc-vjpf, an advisory without a patched release. On 2026-11-18 the
repository owner chose to remove the gate rather than accept that advisory for it.

## What Changes

- **BREAKING (repository tooling):** the `check-mermaid` gate, its adapter `tools/ci/markdown/diagrams.mjs` and
  the devDependency `@mermaid-lint/core` with its jsdom override are removed. Diagram syntax is no longer
  validated; Prettier still checks the fenced blocks' layout, and the forges render the diagrams as before.
- The npm lock loses 142 packages and changes no other; picomatch 2.3.2 leaves it. The `katex` override stays, since
  `micromark-extension-math` still resolves KaTeX through it.
- The dependency policy loses its license corrections for packages that no longer resolve.
- The text policy and the change policy stop naming a diagram validator.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. No accepted specification names the Mermaid gate, so this Change declares `skip_specs: true`.

## Impact

`package.json`, `package-lock.json`, `tools/ci` (the gate, its registration in the quality graph, its tests),
`.config/checks/dependencies/policy.toml`, `docs/governance/text-layout.md`,
`docs/governance/change-and-release-policy.md` and `CHANGELOG.md`.

- **Authority boundary:** the quality graph stays the one owner of which gate covers which carrier; every
  concern that the Mermaid gate covered (documentation, schema) is still covered by another gate.
- **Non-goals:** picomatch 4.0.7, which OpenSpec brings and the same advisory names; its disposition is
  ETHOS's.
