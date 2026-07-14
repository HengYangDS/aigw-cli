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

A projection is an owned, re-creatable output—not an independent source of
truth. Do not alter Codex JSONL, SQLite, archived conversations, model metadata,
or a local proxy deployment to make a configuration test pass.

## Boundary

- Codex Desktop owns the model chosen by each existing conversation and its
  transcripts.
- AIGW owns marked provider blocks, endpoint selection, credentials, and the
  atomic projection across configured Codex targets.
- Codex DMX Proxy owns local Responses transport compatibility and listener
  lifecycle. AIGW must not start, stop, reload, or configure its process.

## Required verification

```bash
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal tools)"
sh scripts/check-governance.sh
python3 scripts/check-markdown-presentation.py
sh scripts/test-changelog.sh
```

Use `aigw sync --dry-run --json` before a configuration mutation where a target
is drifted or a multi-target projection needs review. It must remain credential-
free and must not restart any client.
