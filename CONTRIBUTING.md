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

### Analyzer isolation

Read-only analyzers may inspect `main`. A write-capable analyzer must run in an
isolated non-`main` worktree with a private per-task `TMPDIR`; formatting,
auto-fix, source rewriting, and generated scratch output must not target
`main`. Promote analyzer changes only through reviewed commits in the owned
worktree. Read-only reports, API metadata, ref inventories, and other scratch
data belong below `${TMPDIR:-/tmp}` and must be removed after use; never
redirect them into a checkout or the user home directory.

Before retiring an analyzer worktree, identify its owning task and prove that
the owner handed off or terminated and no owning task remains live. Then apply
the ordinary branch-closeout requirements below. Agent-list visibility alone
is not liveness or retirement proof.

```bash
go run ./tools/architecture --root .
sh scripts/checks/governance/check-module-identity.sh
go run ./tools/coveragegate --race
go vet ./...
sh scripts/checks/quality/check-static-analysis.sh
sh scripts/checks/governance/check-portability.sh
sh scripts/tests/governance/test-portability.sh
test -z "$(gofmt -l cmd internal tools)"
sh scripts/checks/governance/check-governance.sh
AIGW_GITLAB_AUTHOR_EMAIL='<release actor email>' AIGW_GITLAB_ALLOWED_SIGNERS='<path>' sh scripts/checks/forge/check-commit-provenance.sh . gitlab
sh scripts/tests/forge/test-commit-provenance.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/tests/forge/test-replay-history.py
AIGW_TAG_NAMESPACE_FORGE='<local|gitlab|github>' AIGW_GITLAB_ALLOWED_SIGNERS='<path>' AIGW_GITHUB_ALLOWED_SIGNERS='<path>' sh scripts/checks/forge/check-tag-namespace.sh
python3 scripts/checks/governance/check-markdown-presentation.py
python3 scripts/checks/governance/check-text-layout.py
sh scripts/tests/governance/test-text-layout.sh
sh scripts/tests/governance/test-changelog.sh
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
its tag date; run `sh scripts/checks/governance/check-changelog.sh` before requesting review.
GitLab **Project Name** is `AIGW CLI`; stable clone **Path** is `aigw-cli`. Do
not change external paths as a display-name cleanup.

GitLab and GitHub are equivalent, independent Forge planes. Each plane receives
its publication actor and trust material from protected execution context; the
product does not select a maintainer, email, key, or account. Do not copy or
overwrite signed tags between providers. From a clean owned canonical checkout, run
`sh scripts/forge/lib/project-github-forge.sh` to project a branch into the GitHub identity
domain. It maps the existing GitHub tip to an equal canonical tree, appends each
later source commit with the GitHub identity and trusted signature, and uses an
ordinary fast-forward push. It never rewrites history or pushes a tag. Do not
force-push, create snapshot commits, or delete remote refs to manufacture
convergence.

Set `AIGW_GITLAB_AUTHOR_EMAIL`, `AIGW_GITHUB_AUTHOR_NAME`,
`AIGW_GITHUB_AUTHOR_EMAIL`, and `AIGW_GITHUB_SIGNING_KEY` through protected
release context and, for an encrypted key, the approved
`AIGW_GITHUB_SIGNING_PROGRAM`; repository-local `aigw.githubSigningKey` and
`aigw.githubSigningProgram` provide the equivalent persistent configuration.

Every descendant after the tracked provider floor must use its provider email
for both author and committer and verify under that provider's SSH trust anchor.
Keep coverage policy in `.config/checks/coverage/policy.toml`; each Go package
under `./...` and the aggregate must execute strictly above 95 percent. Do not
introduce source compatibility shims, forwarding wrappers, alias-only packages,
or re-exports in place of a semantic owner.

Steady-state forge verification is distinct from delivery-branch closeout.
After explicitly refreshing the required remote-tracking refs without pruning
tags, run the read-only mirror checker:

```sh
git fetch --no-prune --no-prune-tags --no-tags origin \
  refs/heads/main:refs/remotes/origin/main
git fetch --no-prune --no-prune-tags --no-tags github \
  refs/heads/main:refs/remotes/github/main
sh scripts/checks/forge/check-forge-sync.sh \
  --canonical main \
  --peer gitlab:refs/remotes/origin/main:commit \
  --peer github:refs/remotes/github/main:tree
```

The checker never fetches or writes refs. Its result proves only the refs named
by the caller: remote freshness comes from the preceding successful fetches,
while tag provenance, release records, and artifact bytes retain their separate
verification gates.

The public GitHub distribution peer may enforce host rules independently, but
release acceptance never relies on a Forge-specific tag ruleset.
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
sh scripts/checks/forge/check-branch-closeout.sh \
  --source work/<delivery-branch> \
  --canonical main \
  --peer gitlab:refs/remotes/origin/main:commit \
  --peer github:refs/remotes/github/main:tree
```

If a peer cannot be reached, do not invent a matching ref. Record that failed
probe, verify the remaining reachable planes, and defer the unavailable peer's
publication or remote-ref cleanup.
