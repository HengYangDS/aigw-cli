## Why

On 2026-11-13 the owner decided that every governed repository's supply chain moves to the latest stable release, and
that no release is taken before it is 24 hours old ("写进规则，按 24 小时执行"). AIGW's policy held only npm
releases, for three days. Go modules, Mise tools, Actions and images moved at release: that morning's refresh pinned
uv 0.13.8, gh 2.104.0, goreleaser 2.20.2 and the dependency tool's image when they were 3 to 13 hours old. The same
refresh also adopted markdownlint 0.42.1, which requires the fixed re2js itself, and left the override in place
that `supply-chain` says must leave with it.

## What Changes

- Hold every release for one day in the dependency policy: Go modules, Mise tools, Actions, images and the Hermes
  release tag. npm keeps three days. A security fix still skips the age.
- State the one-day floor in the `supply-chain` requirement, and say what happens when a registry publishes no
  release time.
- Remove the re2js override that markdownlint 0.42.1 made unnecessary.
- Run the candidate calculation under the new rule and record what it admits and what it holds.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: a release is at least one day old when it is locked, three days from npm, and a registry without
  release times holds its releases until their publisher's record dates them.

## Impact

The dependency policy, its test, the `supply-chain` specification, the dependency section of the change and release
policy, and the npm manifest. No product code changes. A new release of a Go module, tool, Action or image now
reaches the repository one day later than before.
