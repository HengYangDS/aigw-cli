## Why

On October 19 the operator asked to bring the whole supply chain to its latest
stable releases. Renovate's candidate calculation with the repository policy
found 14 of the 55 direct inputs behind a newer admissible stable release; every
other direct input is current.

## What Changes

- Advance Node.js 26.12.0, uv 0.13.1, glab 1.123.0, golangci-lint 2.15.0 and the
  Renovate image 44.167.5 in the Mise declaration, and regenerate its lock.
- Advance OpenSpec 1.14.2, js-yaml 5.5.0, markdownlint 0.42.0 and the jsdom
  override 30.1.3 in the npm graph.
- Advance `github.com/godbus/dbus/v5` to v5.2.4.
- Advance the CUE-pinned Mise image 2026.10.14 with the digests of the
  runner-owned Windows ARM64 Mise release, `jdx/mise-action` v5.2.0 and the
  native client pins `@openai/codex` 0.163.0 and
  `@anthropic-ai/claude-code-win32-x64` 2.1.301, and regenerate the peer
  projections. Replace the Windows runner's Mise before GitLab consumes them.
- Repair the three tests that golangci-lint 2.15.0 reports.
- Add no updater, wrapper, override or product behavior.

## Capabilities

### Modified Capabilities

- `product-control-plane`: the latest-stable supply-chain requirement gains the
  runner-owned tool scenario that this refresh exercises.

## Impact

Dependency declarations, their locks, the generated CI projections, three tests
and the Windows CI runner's Mise executable change. The D-Bus module is a product
dependency of the Linux Secret Service backend, so the next release's bytes
change; its behavior must not. Installed AIGW, user configuration, credentials
and client projections remain outside this Change.
