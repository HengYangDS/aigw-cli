# Agent Entry Points

This repository is **AIGW CLI**, a local control plane for provider Accounts,
credentials, Profiles, Routes, and explicit Claude/Codex client integration.
It does not run a proxy, listen on a port, carry API traffic, or own Codex
conversation state.

## Canonical Surfaces

- [Project overview and setup](README.md)
- [Contribution and verification workflow](CONTRIBUTING.md)
- [Documentation root](docs/README.md)
- [Authority and projection boundary](docs/architecture/authority-and-projection-boundary.md)
- [Change and release policy](docs/governance/change-and-release-policy.md)
- [Decision register](docs/decisions/README.md)
- [DR-0001](docs/decisions/dr-0001-control-plane-data-plane-boundary.md)
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
- AIGW neither depends on, configures, nor verifies foreign applications or
  their private runtime state.
- AIGW owns marked provider blocks, endpoint selection, credentials, and the
  atomic projection across configured Codex homes shared by CLI and Desktop.
- AIGW does not own Codex Desktop-only GUI settings.
- The admitted client set is Claude Code and Codex. Missing clients remain
  untouched; Hermes and any future client require a separately admitted
  Adapter rather than reuse of Claude or Codex surfaces.
- External Responses compatibility services, when explicitly selected by an
  operator, own their transport and lifecycle. AIGW treats them as ordinary
  Account endpoints and must not install, start, stop, reload, or configure
  them.
- The governed deployment accepted for this closeout selects Codex Responses
  Proxy endpoints for Codex traffic to UCloud, DMXAPI, and AIHubMix. This
  deployment fact does not give AIGW Proxy lifecycle ownership or permit a
  fixed product name, path, port, or process assumption in source.

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
  alias-only packages, or re-exports. Claude Code uses its official user
  settings projection and credential-helper contract; AIGW never intercepts
  the `claude` command or mutates shell profiles.
- Every commit reachable from a published branch tip must use the publication
  actor supplied by that Forge's protected release context and a trusted
  signature from its explicit trust input. No floor, mailmap, or suffix-only
  exception is permitted. Published history may be rebuilt only for an
  explicitly authorized identity repair across both Forge-specific histories,
  tags, releases, and integrity evidence as one fail-closed operation; a partial
  rebuild is never an accepted state.
- Native source verification on macOS, Linux, and Windows blocks trusted CI
  changes and RC releases. Cross-compilation and package inspection cover
  additional CPU targets but do not replace those native source runs. Rooted
  macOS package-lifecycle acceptance remains a GA requirement.

## Required verification

```bash
go run ./tools/ci source
go run ./tools/forge commits --provider gitlab --email '<release actor email>' --allowed-signers '<path>'
go run ./tools/forge tags --mode local --gitlab-allowed-signers '<path>' --github-allowed-signers '<path>'
```

Use `aigw sync --dry-run --json` before a configuration mutation where a target
is drifted or a multi-target projection needs review. It must remain credential-
free and must not restart any client.
