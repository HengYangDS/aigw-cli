# Contributing to AIGW CLI

## Scope

AIGW is a local configuration control plane. Preserve its boundaries: it owns
AIGW-marked Codex configuration projections, not Codex session history or a
proxy process. Never repair a routing problem by editing JSONL, SQLite, model
metadata, an archived transcript, or a third-party gateway deployment.

## Working method

Use an isolated worktree; do not modify a user-owned dirty checkout. Add a
failing regression before changing behavior. Changes to projection logic must
cover successful convergence, preflight rejection, write failure, byte-exact
rollback, and absent-sidecar restoration.

```bash
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal tools)"
sh scripts/check-governance.sh
sh scripts/test-changelog.sh
```

## Projection changes

`aigw sync --dry-run --json` is a read-only planning surface. It may resolve
configuration but must not bind credentials, restart a client, modify a Codex
session, or write config/sidecar state. `aigw sync` prepares every configured
Codex target before its first write and rolls every target back if a commit
fails.

## Release and metadata

Use focused Conventional Commits. Keep `CHANGELOG.md` with `## [Unreleased]` as
its first release section, containing only changes after the latest tagged
release. Every published heading must map to an existing `v<semver>` tag and
its tag date; run `sh scripts/check-changelog.sh` before requesting review.
GitLab **Project Name** is `AIGW CLI`; stable clone **Path** is `aigw-cli`. Do
not change external paths as a display-name cleanup.
