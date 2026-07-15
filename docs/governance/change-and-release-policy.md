# Change and Release Policy

Status: canonical.

## Configuration mutations

All Codex target changes use one transaction: prepare every target, commit only
prepared outputs, and restore every pre-state if any write fails. A partial
projection is a failed outcome, not a tolerable intermediate state.

## Release identity and chronicle

`CHANGELOG.md` begins with `## [Unreleased]`, which contains only changes made
after the latest reachable release tag. The next section must be the latest
reachable `v<semver>` tag, written as `## [<semver>] - <tag-date>`. Every older
published section is likewise anchored to a real tag and its creation date.
Planned versions, branch names, and inferred GA milestones do not belong in the
release chronicle. `scripts/check-changelog.sh` enforces this invariant in CI.

A release tag records a source version; it is not by itself proof of artifact
publication, native-platform acceptance, signing, notarization, or GA. Those
claims require their corresponding evidence and must never be implied by a
Changelog heading.

GitLab **Project Name** is `AIGW CLI`. The stable repository **Path** is
`aigw-cli`. Display text and external identifier are different contracts.

## Distribution continuity

GitLab and GitHub are equal independent forge planes. Each holds the same
commits, branches, signed tags, and versioned 15-artifact release identity:
platform packages, `checksums.txt`, and SPDX SBOM. Each CI/CD plane can build
and publish independently. When both releases are reachable, tag, manifest, and
artifact disagreement is a fail-closed condition; one forge must never bypass
an integrity, provenance, metadata, or version failure observed at the other.

A verified local candidate is a complete extracted artifact directory with one
platform-matching portable archive and a validating checksum record. It exists
for offline acceptance, installation, and rollback verification. A source
checkout, a loose binary, and a tag are not candidates.

## Forge synchronization

The two forges must share the same commit and tag objects. Use
`sh scripts/forge-peer-sync.sh --check` to inspect `main`, then explicitly run
`sh scripts/forge-peer-sync.sh --sync` only from a clean owned worktree to
fast-forward each reachable peer with the exact local commit. The command never
creates snapshot commits, force-pushes, prunes, or deletes refs. A divergence or
a conflicting peer is a manual resolution condition, not a reason to rewrite
history.

## Branch closeout

Merged source branches are disposable delivery artifacts, not project history.
GitLab must enable automatic source-branch deletion after merge. Direct release
or maintenance merges must delete their remote source branch in the same
closeout operation. A branch or worktree may be removed only when its tip is reachable from the
local `main` and every required reachable peer, its worktree is clean, and it
is not `main` or an active unmerged delivery branch. Remove the worktree before deleting its local
branch. Preserve release tags as immutable provenance.

## Cross-project boundary

AIGW manages marked provider configuration and native credential binding only.
Codex DMX Proxy manages its executable payload, manifest, watchdog, and
listener. Neither project may silently adopt the other's state or lifecycle.
