# Forge Operations

Status: canonical.

## Model

GitLab and GitHub are equal independent forge planes. A managed repository uses
one GitLab remote and one GitHub remote; both contain the same commits, branches,
signed tags, and release identity. Each provider owns its own CI/CD execution
and release publication. No provider-specific snapshot commits are permitted.

## Local enrollment

The host-level `agent-forge-peers` tool deliberately has no discovery or
implicit enrollment. It manages only explicitly enrolled, owned repositories in
`~/.config/agent/forge-peers.ini`. Do not enroll third-party clones, temporary
fixtures, foreign worktrees, or a repository without both owner-provided remote
endpoints.

```ini
[repo "example"]
path = /absolute/path/to/example
gitlab_remote = origin
github_remote = github
branches = main
tags = true

[defaults]
# Optional markers for self-hosted or test forge URLs.
gitlab_url_markers = gitlab.example.com
github_url_markers = github.example.com
```

Use `agent-forge-peers status` for a read-only audit. It bounds each Git network
operation and reports `converged`, `behind`, `ahead`, `diverged`, or
`unavailable` for branches, plus signed-tag agreement when tag auditing is
enabled. `agent-forge-peers sync --repo example` requires a clean owned
worktree and only fast-forwards a reachable peer with the exact local commit.
It never creates commits, force-pushes, prunes, or deletes refs.

## Release behavior

A signed tag triggers independently complete GitLab and GitHub pipelines. Each
builds the full matrix from the tagged commit and publishes its own immutable
release. If an identical GitHub release already exists, its assets are
downloaded and byte-verified; a disagreement fails closed rather than replacing
an asset.

A released AIGW binary can contain both source identities. One reachable forge
is sufficient for update availability. When both are reachable, the updater
requires matching latest tags and matching checksum-verified current-platform
artifact bytes before installation.

## Provider identities

GitLab provenance uses `heng.yang.ds@hotmail.com`; GitHub provenance uses
`hengyang.2003@tsinghua.org.cn`. A direct push guard rejects a provider/email
mismatch. Equal-object synchronization is appropriate only where both providers
are deliberately carrying identical commit and tag objects. Where a GitHub
privacy projection exists, run `sh scripts/project-github-forge.sh` from a clean
canonical checkout instead; its isolated rewrite sends only the projected
GitHub branch and leaves both providers' signed tags immutable.
