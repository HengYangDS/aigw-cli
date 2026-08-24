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
mise exec --locked -- go run ./tools/ci source
mise exec --locked -- go run ./tools/forge commits --email '<product author email>' --allowed-signers '<path>'
mise exec --locked -- go run ./tools/forge tags --allowed-signers '<path>'
```

## Projection changes

`aigw sync --dry-run --json` is a read-only planning surface. It may resolve
configuration but must not bind credentials, restart a client, modify a Codex
session, or write config/sidecar state. `aigw sync` prepares every configured
Codex target before its first write and rolls every target back if a commit
fails.

The projected Codex model catalog is the one projection whose effect only the
client itself can confirm, because only the client can report which model
metadata it selected. Changing that projection, or qualifying a new client
build, requires running the verification command against a real installation
and recording the client version and checksum it printed:

```bash
mise exec --locked -- go run ./tools/modelcatalog -model '<provider-prefixed model id>'
```

It asks the client only which input it would send, through a throwaway client
home, so it makes no model request and leaves the machine's own Codex
configuration untouched. Exit code 2 means the client is missing, which is a
prerequisite to satisfy rather than a passing or failing verification. Every
catalog decision a fake client can pin is covered by the package tests instead,
which always run.

The on-screen metadata miss the client reports is a separate matter. The client
announces it only where a person can see it, and reproducing that announcement
non-interactively requires supervising the client's own process tree, which is
outside this repository's scope. Treat its disappearance as an observation to
record with the client version that produced it, not as something this command
measures.

## Release and metadata

Use focused Conventional Commits. Keep `CHANGELOG.md` with `## [Unreleased]` as
its first release section, containing only changes after the latest tagged
release. Every published heading must map to an existing `v<semver>` tag and
its tag date; run `go run ./tools/repository --root . changelog` before requesting review.
GitLab **Project Name** is `AIGW CLI`; stable clone **Path** is `aigw-cli`. Do
not change external paths as a display-name cleanup.

Local Git owns one signed product commit and annotated tag. GitLab and GitHub
are equivalent, independent, optional publication peers that receive those
exact objects. Product signing and trust use `AIGW_RELEASE_AUTHOR_EMAIL` and
`AIGW_RELEASE_ALLOWED_SIGNERS_FILE`; peer transport authentication remains in
Git, SSH, or the protected host credential context. No peer-specific actor,
signing key, tag namespace, history replay, or tree-only equivalence is valid.

From a clean canonical checkout, `go run ./tools/forge project` publishes
`main` atomically to peer `main` and `dev`, or one explicit `proposal/*` to its
matching ref. Candidate, work, and arbitrary branches are rejected. Ordinary
fast-forward and idempotent publication need no destructive option. A divergent
one-time cutover requires every exact observed remote tip and
`--force-with-lease`; restore protected-branch force push immediately after the
post-push observation. See [Forge Operations](docs/operations/forge-operations.md).

The locked repository toolchain is declared once in `mise.toml` and resolved
by `mise.lock`: Go, Node.js, OpenSpec, Prettier, markdownlint, lychee,
GoReleaser, and Syft. Use
`mise exec --locked -- ...` for every repository command. Do not rely on a
system `go`, `node`, `openspec`, `lychee`, `goreleaser`, or `syft` installation.

Protected CI supplies `AIGW_RELEASE_AUTHOR_EMAIL`,
`AIGW_RELEASE_ALLOWED_SIGNERS`, and the generated
`AIGW_RELEASE_ALLOWED_SIGNERS_FILE`. GitLab additionally owns
`CI_API_V4_URL`, `CI_PROJECT_ID`, `CI_COMMIT_TAG`, and `CI_JOB_TOKEN`; GitHub
owns `GITHUB_API_URL`, `GITHUB_REPOSITORY`, `GITHUB_TOKEN`, and `GH_TOKEN`.
These are execution inputs, never product defaults or repository identity.

Every reachable product commit must preserve its local author and committer
email and verify under the explicit product SSH trust anchor.
Keep coverage policy in `.config/checks/coverage/policy.toml`; the aggregate
must exceed 95 percent independently for statement and branch coverage. Every
Go package under `./...` remains mandatory, executed, and visible with exact
diagnostic ratios. Do not
introduce source compatibility shims, forwarding wrappers, alias-only packages,
or re-exports in place of a semantic owner.

Verify peer state directly with current `git ls-remote` observations. A branch
or tag is synchronized only when its full OID equals the local product object.
Hosted Release records and artifact bytes retain separate verification gates.

## Merge closeout

Merge is not the end of a branch lifecycle. After the target branch contains
the source commit, delete the source branch immediately. GitLab is configured
to remove merge-request source branches automatically; for direct, signed
release merges, remove the corresponding remote branch explicitly. Before
removing any branch or worktree, prove all four conditions:

1. its tip is reachable from local `main`;
2. every reachable selected peer contains that exact commit;
3. its worktree is clean and no longer needed; and
4. it is neither `main` nor an active, unmerged delivery branch.

Retire the worktree before its local branch. Tags remain release evidence and
are not branch residue. A locally unreachable peer is not evidence of absence:
record the failed probe and defer only that peer's publication or remote-ref
cleanup.

If a peer cannot be reached, do not invent a matching ref. Record that failed
probe, verify the remaining reachable planes, and defer the unavailable peer's
publication or remote-ref cleanup.
