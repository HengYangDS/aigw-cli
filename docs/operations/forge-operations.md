# Forge Operations

Status: canonical.

## Model

GitLab and GitHub are independent, complete forge planes. A managed repository
uses one GitLab remote and one GitHub remote; each provider owns its own commit
history, signed tags, CI/CD execution, and release publication. The same release
version must have an equivalent 15-artifact matrix, checksums, and SBOM on both
providers, but provider histories and tags are distinct provenance objects. No
provider-specific snapshot commits are permitted.

## Synchronization

The canonical GitLab checkout is projected to GitHub with:

```bash
sh scripts/project-github-forge.sh
```

The command requires a clean canonical checkout, uses a fresh isolated clone,
verifies both providers' post-floor commit identities and each GitHub release
tag whose source tree is present on the selected canonical branch, and also
verifies a same-named canonical GitLab tag. It maps the existing GitHub tip to
an equal canonical tree and appends later source commits with the GitHub email
and trusted signature. The update is an ordinary fast-forward push. It never
alters canonical refs, rewrites history, copies provider tags, or deletes refs.
It uses the repository-local GitHub remote exactly as configured, so user-global
Git URL rewrites cannot silently change its authentication transport. GitLab
recovery uses a normal non-force push of its canonical history once the GitLab
remote is reachable.

Provide the GitHub signing key through `AIGW_GITHUB_SIGNING_KEY` or
`aigw.githubSigningKey`. Encrypted keys additionally use the approved
`AIGW_GITHUB_SIGNING_PROGRAM` or `aigw.githubSigningProgram`; the program must
be executable and remain inside the workstation credential boundary.

Do not use an equal-object branch or tag synchronizer for this repository; its
provider-specific identity model intentionally makes those objects different.

## Mirror verification

A steady-state mirror requires the canonical local branch and GitLab to resolve
to the exact same commit. GitHub must preserve the canonical branch's complete
ordered source-tree history, even though its identity rewrite produces different
commit IDs. Refresh only the required tracking refs, with pruning disabled, and
then run the offline checker:

```bash
git fetch --no-prune --no-prune-tags --no-tags origin \
  refs/heads/main:refs/remotes/origin/main
git fetch --no-prune --no-prune-tags --no-tags github \
  refs/heads/main:refs/remotes/github/main
sh scripts/check-forge-sync.sh \
  --canonical main \
  --peer gitlab:refs/remotes/origin/main:commit \
  --peer github:refs/remotes/github/main:tree
```

The checker performs no network access and writes no refs. It validates the
already-refreshed refs supplied by the caller; it cannot turn a stale tracking
ref into current remote evidence. `check-branch-closeout.sh` remains a separate
retirement proof for a delivery branch and intentionally accepts a canonical
peer that contains that source tip. It is not a steady-state mirror audit.

Qualified `github/*` tags in a canonical checkout are an on-demand provenance
cache owned by the GitHub plane, not by `origin`. Any fetch that can touch tags
in such a checkout must explicitly disable both branch and tag pruning. Never
copy provider-native tags to manufacture symmetry. A retired tag listed in the
tracked retirement inventory is a governed lifecycle difference, not a mirror
gap. For every version active on both forges, verify the complete release asset
set against each provider's independent SHA-256 metadata; matching tag names or
matching checksum manifests alone do not prove matching artifact bytes.

## Dependency transport

Each forge has an independent CI dependency path. GitLab's Go jobs default to
`https://goproxy.cn|https://proxy.golang.org|direct`: the pipe separators permit
the Go toolchain to advance after a transient proxy failure, including a TLS
timeout. A caller may override the complete chain with `AIGW_GOPROXY`; an
override is responsible for preserving the intended fallback semantics. GitHub
Actions does not inherit this GitLab-specific setting.

Every self-hosted runner registration, LaunchAgent, work directory, cache, and label belongs
to exactly one `forge × repository × privilege` tuple. No GitHub runner serves
GitLab jobs, no runner is shared by repositories, and a release runner never
executes verification or merge-request workflow code. AIGW uses three separate
registrations: GitLab macOS is `aigw-release-macos-arm64`, GitLab Windows is
`windows`; GitHub verification is
`aigw-github-verify-macos-arm64`; GitHub release is
`aigw-github-release-macos-arm64`. GitHub verification runs only trusted `main`,
tag, and manual workflows; GitLab remains the canonical merge-request gate.
GitHub-hosted Linux and Windows runners provide independent native operating-
system evidence. Each self-hosted registration has its own service, work
directory, cache, and credential.

The registered GitLab Windows runner is not scheduled because its host shell is
not administratively manageable. GitHub Linux and Windows jobs block trusted
`main`, tag, and manual workflows. The macOS verification runner likewise
blocks trusted source workflows, while release jobs add package-lifecycle
acceptance. No scheduled native job is an allowed failure.

## Release behavior

A release is complete only after the same version has independently completed
both GitLab and GitHub tag pipelines and release publication. A signed tag
triggers each forge's own pipeline. Each
uses the dedicated macOS arm64 release runner class, the exact Go patch version
declared in `go.mod`, the tracked forge-source manifest, and the release epoch
derived from the source-controlled Changelog heading. Each proves two full-matrix builds byte-identical,
then publishes its own independently signed release. If an identical release
already exists, its exact 15 links are verified, every asset is downloaded, and
the complete matrix is compared byte-for-byte with the local checksums and
files. Missing, extra, duplicate, or changed assets fail closed, and the
existing Release is inspected with GET only rather than replaced or updated.
Redirected downloads send the GitLab job token only to the configured GitLab
origin; a cross-host asset store receives no GitLab credential. If one forge is
unavailable, retain a bounded pending-publication record outside `CHANGELOG.md`;
do not create a lasting one-sided release exception.
On the private GitHub Free peer,
tag immutability is not asserted as a host capability: remote tag-signature
verification and cross-forge artifact comparison remain the acceptance proof.

A released AIGW binary contains independent GitLab and GitHub source tuples.
The updater queries every configured peer. If both peers are reachable, their
latest tag must match, then their current-platform artifact bytes must match;
otherwise the update fails before replacing the program. If one peer is
unreachable, the reachable peer may supply the update. Authorization, 4xx,
malformed metadata, missing assets, checksum, archive-validation, downgrade,
and redirect-security failures are terminal. Each provider's tag, manifest, and
artifact remain a single-provider unit.

## Provider identities

GitLab provenance uses `heng.yang.ds@hotmail.com`; GitHub provenance uses
`hengyang.2003@tsinghua.org.cn`. Separate repository-tracked trust anchors
verify every commit after the tracked provider floor and each release tag. A
direct push guard rejects a provider/email or signature mismatch. Same-named tags are independently signed provider provenance records,
and must never be copied, regenerated, or overwritten across the two namespaces.
In a canonical local checkout, GitLab tags remain unscoped and fetched GitHub
provenance is qualified below `github/`; obsolete `provider/` aliases are not
admitted. Native forge checkouts retain their own provider-native `v*` tags.
