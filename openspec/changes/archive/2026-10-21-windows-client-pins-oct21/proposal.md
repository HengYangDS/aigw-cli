## Why

AIGW qualifies real Windows native clients by installing pinned official
packages, `@openai/codex@0.162.1` and
`@anthropic-ai/claude-code-win32-x64@2.1.296`, in the Windows native client
journey. A Renovate `dryRun=full` candidate calculation with the repository
policy on GitHub `dev` (`ec6ddcdc`, 2026-10-21) admits Codex 0.163.0, published
2026-10-15, and Claude Code 2.1.302, published 2026-10-16; both are past the
repository's three-day minimum release age, and npm reports no advisories for
either. Users already run these clients: AIGW 0.4.9 verified Codex CLI 0.163.0
on the operator's machine.

## What Changes

- Pin Codex 0.163.0 and Claude Code 2.1.302 in the CUE authority, with the
  Windows client supplier name that follows from them, and regenerate both
  Forge projections.
- Update the projection tests that name the qualified client set.
- Run the Windows native client journey with the new pins against the
  published predecessor before the forges integrate the change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: the latest-stable supply-chain requirement gains a
  scenario for a qualified native client release.

## Impact

CI declarations, their projections and projection tests change. The AIGW
program, released artifacts, installed AIGW, user configuration and credentials
do not. The GitLab native client journey reads the supplier from the public
input package of each candidate, so the next package carries the new name.
Mise 2026.10.17 stays held: the GitLab Windows runner owns its Mise executable,
and GitLab is unreachable from this workstation.
