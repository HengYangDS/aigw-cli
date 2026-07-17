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

The command requires a clean canonical checkout, uses a fresh isolated clone for
the identity rewrite, verifies overlapping provider tags with their respective
trust anchors, and updates only the selected GitHub branch under a lease. It
never alters canonical refs, copies provider tags, deletes refs, or performs an
unleased force push. It uses the repository-local GitHub remote exactly as
configured, so user-global Git URL rewrites cannot silently change its
authentication transport. GitLab recovery uses a normal non-force push of its
canonical history once the GitLab remote is reachable.

Do not use an equal-object branch or tag synchronizer for this repository; its
provider-specific identity model intentionally makes those objects different.

## Dependency transport

Each forge has an independent CI dependency path. GitLab's Go jobs default to
`https://goproxy.cn|https://proxy.golang.org|direct`: the pipe separators permit
the Go toolchain to advance after a transient proxy failure, including a TLS
timeout. A caller may override the complete chain with `AIGW_GOPROXY`; an
override is responsible for preserving the intended fallback semantics. GitHub
Actions does not inherit this GitLab-specific setting.

## Release behavior

A signed tag triggers independently complete GitLab and GitHub pipelines. Each
uses the dedicated macOS arm64 release runner class, the exact Go patch version
declared in `go.mod`, the tracked forge-source manifest, and the release epoch
derived from the source-controlled Changelog heading. Each proves two full-matrix builds byte-identical,
then publishes its own independently signed release. If an identical release
already exists, its assets are downloaded and byte-verified; a disagreement
fails closed rather than replacing an asset. On the private GitHub Free peer,
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
verify their release tags. A direct push guard rejects a provider/email
mismatch. Same-named tags are independently signed provider provenance records,
and must never be copied, regenerated, or overwritten across the two namespaces.
