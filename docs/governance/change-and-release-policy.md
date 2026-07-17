# Change and Release Policy

Status: canonical.

## Configuration mutations

All Codex target changes use one transaction: prepare every target, commit only
prepared outputs, and restore every pre-state if any write fails. A partial
projection is a failed outcome, not a tolerable intermediate state.

## License

AIGW CLI is distributed under the repository-root [MIT License](../../LICENSE).
All source, documentation, and packaged license references must name the same
license. Third-party dependency licenses are recorded separately in the release
SPDX SBOM.

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

GitLab and GitHub are equivalent independent forge planes. Each has its own
commit history and signed-tag provenance, and each publishes the same versioned
15-artifact release matrix: platform packages, `checksums.txt`, and SPDX SBOM.
Each CI/CD plane can build and publish independently. When both releases are
reachable, tag name, manifest, and artifact disagreement is a fail-closed
condition; one forge must never bypass an integrity, provenance, metadata, or
version failure observed at the other.

A verified local candidate is a complete extracted artifact directory with one
platform-matching portable archive and a validating checksum record. It exists
for offline acceptance, installation, and rollback verification. A source
checkout, a loose binary, and a tag are not candidates.

## Reproducible release inputs

Every formal release matrix has one source-neutral `SOURCE_DATE_EPOCH`: UTC
midnight of the committed `CHANGELOG.md` heading for that exact version. The
package entrypoint rejects a missing or invalid epoch. It normalizes portable
archives, package metadata, MSI identity fields, and the SPDX creation time
before it writes `checksums.txt`.

Before a release plane publishes, it builds the complete 15-artifact matrix
twice with the same version, epoch, and source tuples. The sorted filenames,
every artifact byte, the checksum manifest, and the SPDX SBOM must match. A
native packager that cannot satisfy this contract blocks the tag; it is not
permitted to create a provider-specific exception. The two provider pipelines
then remain independently responsible for their own published-matrix
inspection and cross-forge byte comparison.

## Forge synchronization

GitLab and GitHub remain separate identity domains: GitLab commits and tags use
`heng.yang.ds@hotmail.com`; GitHub projection commits and tags use
`hengyang.2003@tsinghua.org.cn`. Same-named provider release tags are
independently signed provenance objects and must not be overwritten across the
two namespaces.

Run `sh scripts/project-github-forge.sh` from a clean canonical checkout to
project a selected branch into the GitHub identity domain. It rewrites only an
isolated clone, verifies overlapping provider tags against distinct trust
anchors, uses a leased branch update, and never pushes a tag. It honors the
repository-local GitHub URL without inheriting user-global URL rewrites, so its
transport and authentication stay explicit. GitLab recovery uses a normal,
non-force push of canonical history after its remote is reachable. No
equal-object branch or tag synchronizer applies to AIGW.

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
