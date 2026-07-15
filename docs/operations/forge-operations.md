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
unleased force push. GitLab recovery uses a normal non-force push of its
canonical history once the GitLab remote is reachable.

Do not use an equal-object branch or tag synchronizer for this repository; its
provider-specific identity model intentionally makes those objects different.

## Release behavior

A signed tag triggers independently complete GitLab and GitHub pipelines. Each
builds the full matrix from the tagged commit and publishes its own immutable
release. If an identical GitHub release already exists, its assets are
downloaded and byte-verified; a disagreement fails closed rather than replacing
an asset.

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
`hengyang.2003@tsinghua.org.cn`. A direct push guard rejects a provider/email
mismatch. Same-named tags are independently signed provider provenance records,
and must never be copied, regenerated, or overwritten across the two namespaces.
