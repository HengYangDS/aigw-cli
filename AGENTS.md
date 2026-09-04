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
- [Decision register](docs/decisions/decision-register.md)
- [DR-0001](docs/decisions/dr-0001-control-plane-data-plane-boundary.md)
- [Evidence policy](docs/evidence/evidence-policy.md)
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

- `.config/checks/coverage/policy.toml` is the coverage SSOT. Every canonical Go
  package participates, no source or package exclusion is permitted, and the
  policy owns the aggregate floor, package-observation contract, comparison
  semantics, measurement, remediation, and review conditions.
- Keep one semantic owner for each policy and behavior. Prefer cohesive domain
  packages, explicit dependency direction, and narrow interfaces; apply SSOT,
  DRY, MECE, and SOLID rather than duplicating policy across scripts or CI.
- Do not introduce source-level compatibility shims, forwarding wrappers,
  alias-only packages, or re-exports. Claude Code uses its official user
  settings projection and credential-helper contract; AIGW never intercepts
  the `claude` command or mutates shell profiles.
- Every commit reachable from a published branch tip must equal the locally
  constructed product object and verify against an explicit product trust
  input. Object signing and peer transport authentication are separate. A
  Forge may verify and host an object but must never rewrite identity, rebuild
  history, or re-sign a tag.
- Trusted CI changes and RC releases require native source evidence for the
  same product tree on macOS, Linux, and Windows. A Forge may consume that
  product-level evidence without duplicating every executor; its own runner
  availability is an infrastructure signal, not a second product gate.
  Cross-compilation and package inspection cover additional CPU targets but do
  not replace native source evidence. Rooted macOS package-lifecycle acceptance
  remains a GA requirement.
- A claim only a real client can settle must be evidenced by a tracked
  verification command that records the client identity it measured, never by a
  test that skips itself when the client is absent. The Codex model catalog
  projection is verified this way; see [CONTRIBUTING](CONTRIBUTING.md).

## Required verification

```bash
mise install --locked
mise run bootstrap
mise run check
mise run native
mise exec --locked -- go run ./tools/forge commits --email '<product author email>' --allowed-signers '<path>'
mise exec --locked -- go run ./tools/forge tags --allowed-signers '<path>'
```

Use `aigw sync --dry-run --json` before a configuration mutation where a target
is drifted or a multi-target projection needs review. It must remain credential-
free and must not restart any client.
