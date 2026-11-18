## Why

The Mermaid gate validates nine diagrams; eight of them stand in archived Changes and one in current
documentation (`docs/concepts/product-concepts.md`). For that it installs `@mermaid-lint/core`, which brings
Mermaid, jsdom, D3 and 141 further npm packages: 144 of the lock's 249. One of them, picomatch 2.3.2 through
micromatch, is named by GHSA-fc5q-39xc-vjpf, an advisory without a patched release. On 2026-11-18 the
repository owner chose to remove the gate rather than accept that advisory for it.

## What Changes

- **BREAKING (repository tooling):** the `check-mermaid` gate, its adapter `tools/ci/markdown/diagrams.mjs` and
  the devDependency `@mermaid-lint/core` with its jsdom override are removed. Diagram syntax is no longer
  validated; Prettier still checks the fenced blocks' layout, and the forges render the diagrams as before.
- The npm lock loses 144 packages; picomatch 2.3.2 leaves it. The `katex` override stays, since
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
