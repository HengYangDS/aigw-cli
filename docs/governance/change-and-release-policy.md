# Change and Release Policy

Status: canonical.

## Configuration mutations

All Codex target changes use one transaction: prepare every target, commit only
prepared outputs, and restore every pre-state if any write fails. A partial
projection is a failed outcome, not a tolerable intermediate state.

## Release identity and chronicle

`CHANGELOG.md` begins with `## Unreleased`, followed by dated published releases
in descending order. Release claims must distinguish local tests, hosted CI,
physical-platform acceptance, and user-visible conversation recovery.

GitLab **Project Name** is `AIGW CLI`. The stable repository **Path** is
`aigw-cli`. Display text and external identifier are different contracts.

## Cross-project boundary

AIGW manages marked provider configuration and native credential binding only.
Codex DMX Proxy manages its executable payload, manifest, watchdog, and
listener. Neither project may silently adopt the other's state or lifecycle.
