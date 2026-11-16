## Why

"Latest stable repository-owned supply chain" requires each declared input at its latest stable release. A
sweep of every input class on 2026-11-16 (16:41 to 16:44 CST, at `9a3cf734`: Renovate's candidate
calculation with this repository's policy, the Mise tools, the native clients and Hermes) found five
candidates. The dependency Changes on `dev` since 0.8.0 are also unreleased, among them a missing sentence of
`aigw use`.

## What Changes

- Lip Gloss advances from v2.1.0 to v2.1.1.
- The Renovate image advances from 44.239.1 to 44.242.0, pinned to its index digest.
- `jdx/mise-action` advances from v5.2.8 to v5.2.9, pinned to its commit, in the CUE authority and the
  GitHub workflows that it projects.
- The npm lock is refreshed within its declared ranges: DOMPurify 3.4.18 and two packages of `tldts`.
- Two candidates are held for the second time, each with its owner and next action in the design: Mise
  2026.11.8, a runner-owned tool for which this refresh has no window, and `go.yaml.in/yaml/v4`, another
  module, which 34 Go files would have to import in place of v3.
- Release 0.8.1 is prepared.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The refresh applies the accepted supply rules and changes none, so this Change declares
`skip_specs: true`.

## Impact

`go.mod` and `go.sum`, the Mise configuration, the CUE authority and its projections, the npm lock, `VERSION`
and `CHANGELOG.md`.

- **Authority boundary:** each pin stays with its one declared owner; the projections are generated.
- **Reuse:** the existing dependency policy and its checks.
- **Non-goals:** the two held candidates; any change of a supply rule.
