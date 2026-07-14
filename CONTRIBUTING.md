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
python3 scripts/check-markdown-presentation.py
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

## Merge closeout

Merge is not the end of a branch lifecycle. After the target branch contains
the source commit, delete the source branch immediately. GitLab is configured
to remove merge-request source branches automatically; for direct, signed
release merges, remove the corresponding remote branch explicitly. Before
removing any branch or worktree, prove all three conditions:

1. its tip is reachable from `origin/main`;
2. its worktree is clean and no longer needed; and
3. it is neither `main` nor an active, unmerged delivery branch.

Retire the worktree before its local branch. Tags remain release evidence and
are not branch residue.
