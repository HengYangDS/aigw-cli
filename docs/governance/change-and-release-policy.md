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
after the next release candidate is cut. During release preparation, exactly
one next candidate may immediately follow it; after tagging, that heading must
identify the selected `v<semver>` tag. Every older published section is anchored
to a real tag and a valid, source-controlled release date. GitLab and GitHub
sign separate provenance tags, so their tag-object timestamps are not a
cross-forge Changelog invariant. Planned versions, branch names, and inferred
GA milestones do not belong in the release chronicle.
`scripts/check-changelog.sh` enforces this invariant in CI.

When a superseded GitLab release tag is deliberately retired, its Changelog
section remains part of the published product history. The version is recorded
in `packaging/release/retired-gitlab-tags.txt`; the chronology gate requires
that inventory and the retained sections to stay ordered and complete. A
provider that still retains its independently signed historical tag satisfies
the same entry; it is not a duplicate or a reason to rewrite provenance.

A version joins the shared product chronology only after GitLab and GitHub have
each created their own signed provenance tag, completed their CI, and published
their release record. If either forge is unavailable, retain a bounded pending
publication record outside the release chronicle and complete that forge before
calling the version released. A one-sided published-version inventory is not an
accepted steady state.

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

The formal package entrypoint derives the exact Go patch version from `go.mod`,
checks the selected compiler before it writes an artifact, and rejects a
conflicting caller-supplied toolchain. `packaging/release/forge-sources.env` is
the credential-free, source-owned record of both official update peers. Its
resolver validates each complete tuple before packaging and rejects a
conflicting CI or shell override. Thus a release's embedded update topology is
independent of the forge that built it. A direct development `go build` still
has no implicit release source.

Before publication, the complete 15-artifact matrix is built twice on the
dedicated macOS arm64 release runner with the same version, epoch, toolchain,
and manifest. The sorted filenames, every artifact byte, the checksum manifest,
and the SPDX SBOM must match. GitLab and GitHub use that same controlled runner
class, while retaining independent CI/CD, commit, tag, and publication planes.
A native packager that cannot satisfy this contract blocks the tag; no
provider-specific exception is permitted. Both published release matrices are
then downloaded and compared byte-for-byte before either is offered as an
equal update peer.

## Forge synchronization

GitLab and GitHub remain separate identity domains: GitLab commits and tags use
`heng.yang.ds@hotmail.com`; GitHub projection commits and tags use
`hengyang.2003@tsinghua.org.cn`. Each release verifier uses its own tracked
trust anchor. The retired GitHub signer may verify only the explicit legacy
tag inventory; new GitHub release tags must use the current GitHub signer.
Same-named provider release tags are independently signed provenance objects
and AIGW must never copy, regenerate, delete, or overwrite them across the two
namespaces. In a canonical local checkout, unscoped `v*` tags are GitLab
provenance and fetched GitHub tags live only below `github/`; native forge
checkouts keep their own tags unscoped. The `provider/` namespace is retired
and forbidden so a local alias cannot misrepresent its provenance owner.

The private GitHub peer operates on GitHub Free without repository-ruleset tag
protection. Its release tags are therefore signed, independently verified
provenance records, not host-enforced immutable refs. Before a GitHub release
is accepted, fetch the remote tag, verify it against the tracked GitHub trust
anchor, and compare its complete artifact matrix with GitLab. A detected manual
tag change is a provenance failure; it is not an impossible state.

Run `sh scripts/project-github-forge.sh` from a clean canonical checkout to
project a selected branch into the GitHub identity domain. It rewrites only an
isolated clone, verifies every GitHub release tag whose source tree is present
on the selected canonical branch, and retains the separate GitLab verification
for a same-named canonical tag. It uses a leased branch update and never pushes
a tag. It honors the repository-local GitHub URL without inheriting user-global
URL rewrites, so its transport and authentication stay explicit. GitLab
recovery uses a normal, non-force push of canonical history after its remote is
reachable. No equal-object branch or tag synchronizer applies to AIGW.

## Branch closeout

Merged source branches are disposable delivery artifacts, not project history.
GitLab must enable automatic source-branch deletion after merge. Direct release
or maintenance merges must delete their remote source branch in the same
closeout operation. A branch or worktree may be removed only when its tip is
reachable from local `main`, each reachable non-rewriting peer contains that
same tip, and each reachable identity-rewriting projection contains the same
ordered source-tree history. Its worktree must be clean, no longer needed, and
neither `main` nor an active unmerged delivery branch. Remove the worktree
before deleting its local branch. An unreachable peer requires a recorded probe
and a deferred remote closeout; it does not make the local branch a current
delivery lane. Preserve release tags as signed provenance evidence and do not
imply host-enforced immutability where the forge does not provide it.

## Cross-project boundary

AIGW manages marked provider configuration and native credential binding only.
Codex DMX Proxy manages its executable payload, manifest, watchdog, and
listener. Neither project may silently adopt the other's state or lifecycle.
