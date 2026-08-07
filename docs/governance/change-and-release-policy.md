# Change and Release Policy

Status: canonical.

## Decision records and names

- OpenSpec authorizes a bounded change; `docs/decisions/` preserves only its
  durable rationale. It is not a parallel specification or task tracker.
- Project-owned Decision Records use
  `dr-<four-digit-sequence>-<concise-kebab-case-description>.md` with a matching
  `DR-<sequence>` title.
- Project-owned documents, scripts, packages, fixtures, and tests use concise
  names that identify their semantic owner and responsibility. Generic buckets,
  unexplained numeric prefixes, personal identities, and local paths are invalid.
- Ecosystem protocol names remain unchanged, including `README.md`, `go.mod`,
  `main.go`, and OpenSpec carrier names.
- Durable product boundaries, foundational architecture or dependency choices,
  compatibility and retention policy, release trust, security posture, and
  other costly-to-reverse rulings require a Decision Record.

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
after the next release candidate is cut. Published headings are unique, dated
SemVer entries in strict descending order. During release, the first published
heading must identify the selected `v<semver>` tag, and that tag must identify
`HEAD`. Historical headings are the product chronicle; their validity does not
depend on whether a particular Forge still retains an old tag. GitLab and
GitHub sign separate provenance tags, so tag-object timestamps are not a
cross-forge Changelog invariant. Planned versions, branch names, and inferred
GA milestones do not belong in the release chronicle.
`scripts/checks/governance/check-changelog.sh` enforces this invariant in CI.

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

## Go build identity

AIGW is distributed as signed executables and native packages, not as an
importable Go library. Its module declaration is therefore the non-fetchable
product build identity `aigw-cli`; repository-private packages use that identity
only for compilation. It is not a Git remote, organization namespace, homepage,
or installation path, and it must not encode any of them.

All reusable Go implementation stays below `internal/`. Adding a public Go
package is a product-distribution change: it first requires a real, stable,
resolvable organization-owned module path. A personal namespace, private
network coordinate, deployment-specific Forge path, and invented vanity domain
are never acceptable substitutes. The module-identity gate enforces this
boundary and its portability fixtures exercise private, personal, URL, and
filesystem-shaped regressions.

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

## Engineering quality

`.config/checks/coverage/policy.toml` is the coverage SSOT. The executable gate
tests every Go package under `./...` with no exclusion surface and requires each
package and the aggregate to remain strictly above 95 percent. One semantic
owner must govern each behavior and policy; source compatibility shims,
forwarding wrappers, alias-only packages, and re-exports are not admitted
substitutes for cohesive packages, explicit dependency direction, SSOT, DRY,
MECE, and SOLID design.

Native source verification on macOS, Linux, and Windows is blocking on trusted
CI changes and RC releases. Rooted macOS package-lifecycle acceptance is a GA
gate and is not scheduled when the runner lacks a dedicated administrator
credential. A scheduled native source job is never an allowed failure.

## Reproducible release inputs

A release candidate advances to the current stable supported Go compiler, Go
module graph, and CI Actions before it is frozen. `go.mod` owns the exact Go
inputs and `.config/ci/verify-gates.toml` owns immutable Action revisions;
reproducibility repeats those selected versions and is not a reason to preserve
obsolete versions.

Every formal release matrix has one source-neutral `SOURCE_DATE_EPOCH`: UTC
midnight of the committed `CHANGELOG.md` heading for that exact version. The
package entrypoint rejects a missing or invalid epoch. It normalizes portable
archives, package metadata, MSI identity fields, and the SPDX creation time
before it writes `checksums.txt`.

The formal package entrypoint derives the exact Go patch version from `go.mod`,
checks the selected compiler before it writes an artifact, and rejects a
conflicting caller-supplied toolchain. Protected release context owns both
official update-peer tuples; `.config/release/forge-sources.env` is a
fictitious shape fixture only. The resolver validates each complete tuple
before packaging. Thus product source remains independent of the Forge that
publishes it, while a direct development `go build` has no implicit release
source.

Before publication, the complete 15-artifact matrix is built twice on the
protected release runner with the same version, epoch, toolchain, and explicit
Forge coordinates. The sorted filenames, every artifact byte, the checksum manifest,
and the SPDX SBOM must match. GitLab and GitHub use that same controlled runner
class, while retaining independent CI/CD, commit, tag, and publication planes.
A native packager that cannot satisfy this contract blocks the tag; no
provider-specific exception is permitted. Both published release matrices are
then downloaded and compared byte-for-byte before either is offered as an
equal update peer.

## Forge synchronization

GitLab and GitHub remain separate identity domains. Their protected release
contexts supply the publication actors and trust inputs; reusable product code
does not select a person, account, email, key, or signing program. The retired
GitHub signer may verify only the explicit legacy
tag inventory; new GitHub release tags must use the current GitHub signer.
Same-named provider release tags are independently signed provenance objects
and AIGW must never copy, regenerate, delete, or overwrite them across the two
namespaces. In a canonical local checkout, unscoped `v*` tags are GitLab
provenance and fetched GitHub tags live only below `github/`; native forge
checkouts keep their own tags unscoped. The `provider/` namespace is retired
and forbidden so a local alias cannot misrepresent its provenance owner.

The GitHub peer is a public distribution surface. Its release tags remain
signed, independently verified provenance records; release acceptance does not
delegate integrity to an optional host ruleset. Before a GitHub release is
accepted, fetch the remote tag, verify it against the tracked GitHub trust
anchor, and compare its complete artifact matrix with GitLab. A detected manual
tag change is a provenance failure; it is not an impossible state.

Every commit reachable from a published branch tip uses its Forge's explicit
publication actor and trust input. Verification walks the complete reachable
graph; a floor, `.mailmap`, or clean suffix cannot conceal invalid provenance.
GitHub-hosted jobs receive allowed-signers content from protected repository
variables, write it with restrictive permissions to a runner-temporary file,
and pass only that file path to verification. GitLab supplies the equivalent
input as a protected file variable. Neither workflow stores trust material in
the checkout or logs its contents.
If a maintainer explicitly authorizes repair of an already published identity
violation, reconstruct each Forge-specific history in isolated object storage
and replace every affected branch, signed tag, release record, and integrity
receipt together. The repair remains incomplete while either Forge still
exposes invalid or mixed history.

Architecture policy paths use one repository-relative grammar on every runner.
POSIX roots, Windows drive, UNC or device roots, backslashes, empty segments,
dot segments, and parent traversal are rejected independently of the host OS.

Run `sh scripts/forge/lib/project-github-forge.sh` from a clean canonical checkout
to project a selected branch into the GitHub identity domain. It verifies every
reachable canonical and GitHub commit, verifies every
GitHub release tag whose source tree is present on the selected canonical
branch, retains the separate GitLab verification for a same-named canonical
tag, and maps the current GitHub tip to an equal canonical source tree. It then
appends later source commits with their merge topology, the GitHub identity,
and a trusted signature, using an ordinary fast-forward push. It never rewrites
steady-state history or pushes a tag. It honors the repository-local GitHub URL without
inheriting user-global URL rewrites, so transport and authentication stay
explicit. GitLab recovery uses a normal, non-force push of canonical history
after its remote is reachable. No equal-object branch or tag synchronizer
applies to AIGW.

A steady-state synchronization claim requires current, explicitly refreshed
peer refs. The local canonical branch and GitLab peer must have identical commit
IDs; the GitHub projection must preserve the canonical branch's complete
ordered source-tree history. `scripts/checks/forge/check-forge-sync.sh` enforces those
offline ref invariants without fetching or writing. It does not replace
provider-specific tag verification, release-record comparison, or independent
SHA-256 verification of every shared release asset. Matching manifests without
matching provider asset digests are insufficient evidence. Tracked retirement
inventories remain the only admitted explanation for a deliberate
provider-specific historical tag absence.

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
An optional external compatibility service manages its own executable payload,
manifest, supervision, and listener. AIGW composes with it only through an
operator-selected HTTP endpoint; neither product may silently adopt the other's
state or lifecycle.
