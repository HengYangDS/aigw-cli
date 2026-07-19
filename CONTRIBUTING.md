# Contributing to AIGW CLI

## Scope

AIGW is a local configuration control plane. Preserve its boundaries: it owns
AIGW-marked Codex configuration projections, not Codex session history or a
proxy process. Never repair a routing problem by editing JSONL, SQLite, model
metadata, an archived transcript, or a third-party gateway deployment.

## License

By contributing, you agree that your contributions are licensed under the
repository's [MIT License](LICENSE).

## Working method

Use an isolated worktree; do not modify a user-owned dirty checkout. Add a
failing regression before changing behavior. Changes to projection logic must
cover successful convergence, preflight rejection, write failure, byte-exact
rollback, and absent-sidecar restoration.

Local developer-tool state, including `.serena/`, is disposable and ignored.
It may index the current checkout, but it is not AIGW configuration, evidence,
or an input to release and runtime decisions. Do not add it to commits, copy it
between worktrees, or use it to reconstruct source state.

```bash
go test -race ./...
go vet ./...
sh scripts/check-static-analysis.sh
test -z "$(gofmt -l cmd internal tools)"
sh scripts/check-governance.sh
python3 scripts/check-markdown-presentation.py
python3 scripts/check-text-layout.py
sh scripts/test-text-layout.sh
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

GitLab and GitHub are equivalent, independent forge planes. GitLab history and
signed tags use `heng.yang.ds@hotmail.com`; GitHub projection history and signed
tags use `hengyang.2003@tsinghua.org.cn`. Do not copy or overwrite signed tags
between providers. From a clean owned canonical checkout, run
`sh scripts/project-github-forge.sh` to project a branch into the GitHub identity
domain. It rewrites only an isolated clone, applies a leased branch update, and
never pushes a tag. Do not force-push, create snapshot commits, or delete remote
refs to manufacture convergence.

The private GitHub Free peer does not provide repository-ruleset tag protection.
Describe its release tags as signed and independently verified provenance, not
as host-enforced immutable refs. AIGW automation still never updates or deletes
a provider-native release tag.

## Merge closeout

Merge is not the end of a branch lifecycle. After the target branch contains
the source commit, delete the source branch immediately. GitLab is configured
to remove merge-request source branches automatically; for direct, signed
release merges, remove the corresponding remote branch explicitly. Before
removing any branch or worktree, prove all four conditions:

1. its tip is reachable from local `main`;
2. every reachable peer contains the corresponding proven content: the same
   commit for a non-rewriting peer, or the same ordered source-tree history for
   an identity-rewriting projection;
3. its worktree is clean and no longer needed; and
4. it is neither `main` nor an active, unmerged delivery branch.

Retire the worktree before its local branch. Tags remain release evidence and
are not branch residue. A locally unreachable peer is not evidence of absence:
record the failed probe and defer only that peer's publication or remote-ref
cleanup.

After refreshing reachable peer refs, run the closeout verifier from the
canonical checkout. Use `commit` for GitLab's canonical history and `tree` for
GitHub's identity-rewriting projection:

```sh
sh scripts/check-branch-closeout.sh \
  --source work/<delivery-branch> \
  --canonical main \
  --peer gitlab:refs/remotes/origin/main:commit \
  --peer github:refs/remotes/github/main:tree
```

If a peer cannot be reached, do not invent a matching ref. Record that failed
probe, verify the remaining reachable planes, and defer the unavailable peer's
publication or remote-ref cleanup.
