# Agent Entry Points

This repository is **AIGW CLI**, the local control plane for provider account
configuration, credentials, route selection, and Codex/Claude configuration
projections. It does not run a proxy, listen on a port, carry API traffic, or
own Codex conversation state.

## Canonical Surfaces

- [Project overview and setup](README.md)
- [Contribution and verification workflow](CONTRIBUTING.md)
- [Documentation root](docs/README.md)
- [Authority and projection boundary](docs/architecture/authority-and-projection-boundary.md)
- [Change and release policy](docs/governance/change-and-release-policy.md)
- [ADR-0001](docs/decisions/0001-control-plane-data-plane-boundary.md)
- [Evidence policy](docs/evidence/README.md)
- [Release history](CHANGELOG.md)

## Authority Order

1. Current user instruction and explicit lifecycle authorization.
2. Source code, tests, schemas, package metadata, and CI.
3. Canonical documents and decisions under `docs/`.
4. AIGW-owned marked projections and their sidecar state.
5. IDE caches, client runtime state, generated reports, and logs.

A projection is an owned, re-creatable output--not an independent source of
truth. Do not alter Codex JSONL, SQLite, archived conversations, model metadata,
or a local proxy deployment to make a configuration test pass.

## Boundary

- Codex Desktop owns the model chosen by each existing conversation and its
  transcripts.
- AIGW owns marked provider blocks, endpoint selection, credentials, and the
  atomic projection across configured Codex targets.
- Codex DMX Proxy owns local Responses transport compatibility and listener
  lifecycle. AIGW must not start, stop, reload, or configure its process.

## Analyzer isolation

An analyzer may inspect `main` read-only. Any analyzer capable of formatting,
auto-fixing, rewriting, or otherwise writing source must use an isolated
non-`main` worktree and a private per-task `TMPDIR`; it must never auto-fix
`main`. Scratch reports and API or ref inventories must stay in that temporary
directory, be removed after use, and never be redirected into a checkout or the
user home directory. Before retiring its worktree, record the owning task and
prove that the owner has handed off or terminated and that no owning task
remains live.
Worktree visibility or an apparently idle agent is not retirement authority.

## Engineering quality

- `.config/checks/coverage/policy.toml` is the coverage SSOT. Every Go package
  under `./...` participates, no source or package exclusion is permitted, and
  each package and the aggregate must be strictly greater than 95 percent.
- Keep one semantic owner for each policy and behavior. Prefer cohesive domain
  packages, explicit dependency direction, and narrow interfaces; apply SSOT,
  DRY, MECE, and SOLID rather than duplicating policy across scripts or CI.
- Do not introduce source-level compatibility shims, forwarding wrappers,
  alias-only packages, or re-exports. The client launcher manager in
  `internal/shims` is owned product behavior, not permission for forwarding
  architecture.
- `packaging/release/verified-commit-floors.txt` is the forward-only identity
  boundary. Every later GitLab commit must use `heng.yang.ds@hotmail.com` and a
  trusted GitLab signature; every later GitHub projection commit must use
  `hengyang.2003@tsinghua.org.cn` and a trusted GitHub signature. Do not rewrite
  the historical floors or published release tags.
- Native source verification on macOS, Linux, and Windows blocks trusted CI
  changes and RC releases. Cross-compilation and package inspection cover
  additional CPU targets but do not replace those native source runs. Rooted
  macOS package-lifecycle acceptance remains a GA requirement.

## Required verification

```bash
go run ./tools/coveragegate --race
go vet ./...
sh scripts/check-static-analysis.sh
test -z "$(gofmt -l cmd internal tools)"
sh scripts/check-governance.sh
sh scripts/check-commit-provenance.sh . gitlab
sh scripts/test-commit-provenance.sh
sh scripts/check-tag-namespace.sh
python3 scripts/check-markdown-presentation.py
python3 scripts/check-text-layout.py
sh scripts/test-text-layout.sh
sh scripts/test-changelog.sh
```

Use `aigw sync --dry-run --json` before a configuration mutation where a target
is drifted or a multi-target projection needs review. It must remain credential-
free and must not restart any client.
