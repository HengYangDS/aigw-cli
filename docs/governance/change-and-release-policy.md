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

## Independent provider planes

GitLab and GitHub are independent verification and publication planes. Each
runs the complete source and governance gate set from its own workflow. Neither
provider treats the other provider's last successful pipeline as sufficient
release evidence.

GitLab remains the canonical source-history authority. GitHub holds an
email-safe projection of the same source tree and independently validates that
projection. The GitHub mirror is synchronized only with
`git github-mirror-sync`; direct pushes are intentionally disabled. Release
assets may be published only after the local artifact matrix and the provider's
own gate set agree.

## Distribution continuity

GitLab is the formal primary release channel. GitHub is independently operable
only as a mirror of the exact same versioned 15-artifact release matrix:
platform packages, `checksums.txt`, and SPDX SBOM. Mirror availability may
recover from primary transport failure; it must never bypass an integrity,
provenance, metadata, or version failure at the primary source.

A verified local candidate is a complete extracted artifact directory with one
platform-matching portable archive and a validating checksum record. It exists
for offline acceptance, installation, and rollback verification. A source
checkout, a loose binary, and a tag are not candidates.

## Branch closeout

Merged source branches are disposable delivery artifacts, not project history.
GitLab must enable automatic source-branch deletion after merge. Direct release
or maintenance merges must delete their remote source branch in the same
closeout operation. A branch or worktree may be removed only when its tip is
reachable from `origin/main`, its worktree is clean, and it is not `main` or an
active unmerged delivery branch. Remove the worktree before deleting its local
branch. Preserve release tags as immutable provenance.

## Cross-project boundary

AIGW manages marked provider configuration and native credential binding only.
Codex DMX Proxy manages its executable payload, manifest, watchdog, and
listener. Neither project may silently adopt the other's state or lifecycle.
