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

### Closing a repair

Identify the violated contract and its existing owner before choosing a fix.
Distinguish product prerequisites from assumptions introduced by the current
implementation. Repair the owner, exercise its affected consumers, and remove
the superseded path rather than maintaining two answers to the same problem.

Keep the smallest reproducer as a regression. Run focused checks before the
complete gate on stable inputs; a failure returns to its narrow reproducer.
For shipped manifests and generated configuration, also exercise the actual
delivery input through the public command. Small fixtures isolate a cause but
cannot establish that the shipped catalogue works. Native authorization fixtures
must verify the persisted security format and effective enforcement boundary,
not merely recreate an API call in temporary storage. Isolation must preserve the
security semantics exercised by the deployed product.

Permission-sensitive fixtures must set their intended mode explicitly after
creation; creation modes are filtered by the caller's umask. Exercise those
fixtures under both ordinary and restrictive umasks without changing the
process-wide mask inside concurrent tests. Restrict evidence-directory access
with an exact path operation rather than leaking an output-creation umask into
the test process.

Every selected native journey
must consume the explicit candidate and report its binary digest; an absent
candidate is an input failure, not permission to substitute a source build.
Cover deferred prerequisites becoming available independently while preserving
explicit user choices. Compare owned configuration values semantically, then
assert byte-exact preservation of user files when no change is required.

For an update that changes persisted configuration, exercise rollback after the
successor writes that configuration; an unchanged predecessor fixture cannot
prove downgrade safety. Before an expensive matrix build, run the unchanged
historical lifecycle against a verified released predecessor with its generated
configuration still active. A real-client journey that disables an adapter
before replacement proves a different transition; run both through the existing
release acceptance command. Test isolation includes derived native paths, not only
environment variables: staged programs and user-data roots must stay disjoint.

Package-manager verification uses an explicit disposable portable target, even
when the command is expected to reject the request. Never test a rejection by
targeting the operator's default installation. Compare both source and target
ownership, retain the host executable digest, and verify it is unchanged.

Capture each client's credential command, arguments and environment before
replacement, then execute that retained invocation before synchronization or
client configuration reload. A restarted client reading a new helper cannot
prove that an existing caller still works. Native-store acceptance must also
retain the original credential item and identify the actual reader executable
and implementation on both sides; preserving the CLI path is insufficient.

Require complete program/configuration rollback and original-caller acceptance
before a production cutover. A locally authored workaround is not an approved
product dependency, regardless of its name or another task's successful probe.

Record acceptance and evidence references in the active OpenSpec task, update
the relevant operator guidance, and remove contradictory instructions. A new
rule, skill, or passing format check is not proof that the failure cannot recur.
Source, packaged, installed, and hosted outcomes require their own observations.

For specification changes, inspect the current requirement before adding a delta.
Use OpenSpec's MODIFIED operation for a changed contract and REMOVED for an
obsolete duplicate, naming the surviving owner and preserving its obligations.
Review the complete projected specification with the locked OpenSpec merge,
not just the changed paragraphs. Valid delta syntax and preserved scenario
names do not prove that inherited requirements agree with the new behavior.
Once canonical specs absorb a delta, remove its redundant operations only after
the official merger proves that every complete projected spec remains
byte-identical. Keep outstanding deltas, original Change intent and unfinished
tasks; removing consumed delta files does not archive or complete the Change.

Local developer-tool state, including `.serena/`, is disposable and ignored.
It may index the current checkout, but it is not AIGW configuration, evidence,
or an input to release and runtime decisions. Do not add it to commits, copy it
between worktrees, or use it to reconstruct source state.

### Bounded TDD journey

For one semantic change, use this path:

1. Find the current requirement in [OpenSpec](openspec/) and the implementation
   owner through the [architecture boundary](docs/architecture/authority-and-projection-boundary.md).
2. Add the smallest regression at that owner's public or domain boundary, then
   run it and confirm that it fails for the intended missing behavior.
3. Implement the smallest complete repair and delete the superseded path.
4. Re-run the focused test, the affected package or projection checks, and then
   `mise run check` after the semantic closure is stable.
5. Review the exact introduced commit range through
   [Forge object verification](docs/operations/forge-operations.md#verify-local-objects),
   and keep native, hosted, published, and installed evidence separate.

This sequence identifies the invariant, owner, regression, implementation,
gate, and evidence without private workstation context. A formatter-only change
does not need an invented failing behavior test; run its native formatter and
structural gate instead.

For codebase-memory, select an existing project by its exact worktree root,
not a similar name, and reuse it. Check `check_index_coverage` for the relevant
paths before graph queries. Missing coverage metadata or changed file metadata
requires `index_repository` for that root and the same project name, with
`mode: full` and `persistence: false`, followed by a fresh coverage check. A
`ready` status or current Git HEAD alone does not establish index freshness.
Treat caller-resolution confidence as evidence quality, not a correctness
guarantee; verify ambiguous edges against source. Automatic watching is
session-dependent and does not replace this check.

The native [`.cbmignore`](.cbmignore) retains source-owned coverage tooling and
policy that the indexer's directory-name defaults would otherwise skip. Other
generated output and checkout dependencies remain excluded. The index remains
optional developer state, not a build dependency or proof authority.

### Analyzer isolation

Read-only analyzers may inspect `main`. Write-capable analysis runs in an owned
non-`main` worktree with a private per-operation `TMPDIR`. Promote source changes
through reviewed commits; keep generated output separate from tracked source.

Before retiring an analyzer worktree, identify its owning task and prove that
the owner handed off or terminated and no owning task remains live. Then apply
the ordinary branch-closeout requirements below. Agent-list visibility alone
is not liveness or retirement proof.

### Output ownership and cleanup

Paths below are relative to the active worktree unless a tool selects them.

- **Build intermediates and candidates** belong in ignored `build/` and
  `dist/`. Remove them when superseded or published.
- **Pending verification evidence** belongs in
  `build/verification/<source-commit>/`. Retain it until its decision is resolved
  and required evidence has a durable owner.
- **Downloads, extraction and analyzer scratch** use one private hidden
  directory under `build/tmp/`, passed as `TMPDIR`. Reclaim it on success,
  failure or interruption.
- **Dependency caches** belong to the package manager; share them only where
  that tool supports it.
- **ETHOS evidence and coordination** follow the current command's artifact
  reference. ETHOS owns Git-common-dir storage and retention.
- **Published evidence** belongs to the exact revision's CI artifacts and
  release assets. Local-only work retains required native evidence without
  depending on a Forge.

Operating-system temporary storage remains suitable when a tool needs it, but
not for long-lived handoffs. An operation owns cleanup, including after a
failed child process; after a crash, remove only its exact stopped resources.
Before retiring a worktree, preserve evidence still needed by a pending decision
at its existing authoritative owner, then remove disposable output.

Stored source snapshots are evidence, not packages. Save inspected Go source as
`.go.txt` or inside an archive, preserving its original bytes and digest. Run
temporary Go helpers in an isolated scratch directory and retain only their
non-compilable source snapshots. A Git ignore does not isolate package discovery:
`go mod tidy` can still discover ordinary `.go` files under ignored `build/`.
Use a dot-prefixed operation directory such as `build/tmp/.release-check/`:
Go's native package discovery skips it, including while concurrent tests create
and remove nested fixtures. An ordinary ignored directory is not isolation.
Do not add a dependency or weaken a gate to accommodate retained research data.

Keep original command output when a consumer needs it. A handwritten
`receipt.json`, copied verdict, or checksum of an agent summary is not another
proof authority. Reference the tool-native result rather than duplicating it;
external observations must be refreshed when the decision depends on them.

```bash
mise run check
mise run native
```

For signed-object admission, use the exact introduced range in
[Forge object verification](docs/operations/forge-operations.md#verify-local-objects).
That owner distinguishes change admission from whole-history and release-tag
audits.

## Projection changes

`aigw sync --dry-run --json` is a read-only planning surface. It may resolve
configuration but must not bind credentials, restart a client, modify a Codex
session, or write config/sidecar state. `aigw sync` prepares every configured
Codex target before its first write. If a commit fails, it compensates in
reverse order, restoring only targets that still match its own write. Newer
external edits are preserved; see the [transaction contract](docs/decisions/dr-0006-transactional-client-projection.md).

The projected Codex model catalog is the one projection whose loading only the
client itself can confirm. Changing that projection, or qualifying a new client
build, requires running the verification command against a real installation
and recording the client version, executable checksum, and model-entry digests:

```bash
mise exec --locked -- go run ./tools/codex/catalog -model '<provider-prefixed model id>'
```

It asks the client to render the effective catalog through a throwaway client
home, then proves that the provider-prefixed entry is identical to its bundled
base entry apart from `slug`. It makes no model request and leaves the user's
Codex configuration untouched. Exit code 2 means the client is missing, which
is a prerequisite to satisfy rather than a passing or failing verification.
Every deterministic catalog decision is also covered by package tests.

Catalog inspection does not prove that the selected Account, credential, model,
and synchronized projection work through the real client. That quota-consuming
claim has one public authority:

```bash
aigw verify --for codex
```

The command runs the configured Codex executable once with an ephemeral
session, one deterministic synchronized target, and the selected Route. A
successful result reports the measured client version and executable SHA-256;
it does not print the Account Token or model response. Do not replace this
evidence with a direct HTTP probe, a mocked client, or a skipped test when Codex
is unavailable.

Claude has the corresponding public check:

```bash
aigw verify --for claude
```

It first checks that the actual Claude settings match the selected Route and
AIGW credential helper. Missing or stale settings require `aigw sync`; verify
does not repair them or inject a second endpoint, model, or Token. It runs the
real client with those settings in [bare mode](https://code.claude.com/docs/en/headless#start-faster-with-bare-mode),
with tools and session persistence disabled. AIGW-managed Anthropic environment
overrides are removed from that child process; environment credentials remain
available to the projected helper. Bare mode does not read Claude subscription
credentials or its system keychain, but the helper still uses the Account's
chosen credential backend. Use an isolated home and explicit environment
backend for unattended acceptance that must not access a host credential store.

A local protocol fixture can prove that a real client consumed the generated
settings and helper. It does not establish availability of a real Provider.
Record the client version, artifact digest, endpoint class, and precise claim
separately; do not call a fixture-backed success a live Provider acceptance.

## Development and verification

Install the locked repository toolchain and this Work Lane's dependencies
with:

```bash
mise install --locked
mise run bootstrap
```

The [tool declaration](mise.toml) and [tool lock](mise.lock) own language
runtimes and standalone tools. The [npm package declaration](package.json) and
[dependency lock](package-lock.json) own OpenSpec, Prettier, markdownlint,
Mermaid lint, and their complete npm dependency graph. They define
dependencies, not a second command plane.

Use `mise run check` for the complete source gate, `mise run native` for
current-host acceptance, and [`mise run release`](#signed-artifact-builds) for
a signed, deterministic non-publishing build. The Mise tasks delegate execution
to the existing Go owners, which invoke checkout-local package entrypoints
directly.

`bootstrap` first runs `go mod tidy -diff`, then
`npm ci --include=dev --ignore-scripts`. Development dependencies are required
even when the caller sets `NODE_ENV=production` or npm's `omit=dev`; bootstrap
does not change either user setting.

The Go step prepares the complete source and dependency-test graph and rejects
lock drift without rewriting the [Go module declaration](go.mod) or
[dependency checksums](go.sum). Merely compiling AIGW does not populate every
module needed for later dependency resolution. CI calls this same task rather
than maintaining a separate npm-only setup sequence.

The repository's native `mise` environment disables the persistent user Go
environment file and parent workspaces, and uses only the selected bundled
[Go toolchain](https://go.dev/doc/toolchain). `GOENV=off`, `GOWORK=off`, and
`GOTOOLCHAIN=local` apply to both `mise exec --locked --` and repository tasks;
they neither rewrite user settings nor download another compiler behind the
lock. An incompatible compiler fails explicitly. Deliberate process-level build
flags and platform targets remain separate inputs; these settings do not claim
to make an arbitrary shell hermetic. Bare `go` outside the managed environment
is not the repository verification entrypoint.

Node and npm are pinned independently in [mise.toml](mise.toml). The native
`npm` tool name selects the cross-platform `npm:npm` backend and takes precedence
over Node's bundled npm; installing a separate tool without that precedence
does not change the command users execute. The native `.mise/locks/` sidecars
are part of [the tool lock](mise.lock), not a second application dependency
manifest. Keep their generated bytes and digest together in checkout snapshots
and CI evidence; do not reformat the generated dependency lock.

The Go quality graph invokes the installed OpenSpec, Prettier,
markdownlint, and Mermaid entrypoints through the locked Node runtime. It does
not route through `package.json` scripts or platform-specific npm launchers.
Missing checkout-local dependencies fail with an instruction to run
`mise run bootstrap`; global installations are never fallback authorities.

[The native early configuration](.config/miserc.toml) excludes parent, global
and system policy before mise reads tool declarations. Local development and
both Forges consume this same file, not separate environment overrides or an
invented empty CI configuration. Native path templates follow the operator's
configuration locations without changing those files or shared package caches.
Run from the target checkout or a nested directory; `mise -C` alone does not
select another checkout's early configuration. `mise config` shows the actual
inputs. The native regression executes ordinary local commands and all three Forge
projections from root and nested directories, preserves the owned setting and
proves foreign configurations remain unchanged and unselected.

An isolated verification checkout must preserve the measured source's commit,
actual branch role, remote metadata and required release tags. An exact commit
in detached HEAD is not equivalent to the original work branch for lifecycle
checks. Copy existing Git objects and metadata rather than inventing a
publication ref, synthesizing a tag or changing tracked files. Record those
inputs with the result; no verification checkout becomes an authoring lane.

### Environment reconstruction

The ordinary test suite snapshots the current Go source and native mise/Go/npm
inputs, including the early configuration, into a private checkout with spaces
in its path. It re-resolves Go and npm locks and runs the actual bootstrap twice,
using populated dependency caches
offline. Before isolating npm settings, it resolves and retains npm's exact
cache path rather than assuming a platform default. Each pass checks unchanged
input bytes, actual tool versions, and removal of a deliberately stale
installation file. Missing cache content fails rather than silently downloading
or skipping; run the initial bootstrap before testing. These checks prove
repeatable Go/npm resolution and installation, not fresh-download availability
or mise lock refresh. Standalone executables are checked separately against
the [complete tool declaration](mise.toml).

OSV Scanner uses mise's native Go backend so its integrated call analysis can
read the repository's Go language version. Go module checksums authenticate the
source build; upstream binary SLSA attestations do not describe this locally
compiled executable. The native executable-identity test also checks its
compiler against the locked Go version. After changing Go, rebuild a cached
scanner with `mise install --force go:github.com/google/osv-scanner/v2/cmd/osv-scanner`.
Then rerun `mise run check`; do not disable call analysis to accept an old build.

CI tool installation uses Go's HTTP/1.1 transport after repeated HTTP/2 stream
resets from the module and checksum services. The CUE projection scopes
`GODEBUG=http2client=0` to the installer process; product tests and runtime keep
their normal transport. TLS and module checksum verification remain enabled.
This does not select an older prebuilt scanner: its compiler must still support
the repository's Go language version and real vulnerable-source analysis.

### Native lock refresh

Run `mise run dependencies:resolve` from a clean, authorized checkout to refresh
the current host's mise lock metadata twice. Native mise templates select the
OS and architecture. Git compares both manifests with `HEAD` before execution
and after each pass, including staged edits; any drift fails and remains
available for review. This task refreshes metadata, not dependency versions, and
is deliberately separate from offline tests and ordinary bootstrap.

GitHub's **Verify** workflow exposes `refresh_locks` as an optional manual input
alongside the candidate ref and `commit_base`. Each native job supplies its
short-lived workflow token only to this step and retains the resulting lock as
an artifact. GitLab's native jobs invoke the same task when
`AIGW_REFRESH_LOCKS=true`; the runner operator supplies an appropriately scoped
`MISE_GITHUB_TOKEN` for upstream GitHub metadata, independently of the GitLab
repository credential. Native job artifacts retain the observed lock even when
refresh reports drift. A peer's repository credential is not automatically an
upstream release credential.

Keep version discovery and provenance verification enabled. Inspect the native
command log as well as its exit status: upstream warnings that defer verification
do not establish complete provenance. Offline mode is not a substitute for
remote metadata resolution. A successful cache-backed install and a successful
online lock refresh are separate claims.

### CI tool caches

[The CUE model](.config/ci/pipeline.cue) projects native tool caching to both
Forges. Every job still runs `mise install --locked`; cache presence is neither
verification evidence nor permission to skip a gate. `mise run bootstrap`
prepares Go dependencies and rebuilds Node packages in that checkout.

GitHub uses the pinned mise action's cache key, which includes the platform,
runner image, mise version, configuration and lockfile hashes. The job identifier
separates installation scopes. GitLab Linux retains `installs/` and native
`cache/` metadata together beneath `build/runtime/tool-cache/.mise/`, keyed by
both directory roles, pinned container digest, runner, job, branch, and native
manifest and lock hashes. Completion markers belong to that metadata: copying
an interrupted installation without them can make unfinished tools appear ready.
GitLab saves this cache after success or failure, preserving completed tools
when another download or gate fails. Cache retention never changes job status.

The hidden directory keeps tool sources outside Go package discovery. Retain
GitLab's separate protected-branch caches. A restored cache is a job-local copy,
not another lane's live environment; credentials, user configuration,
`node_modules`, product builds and proof results are not cache inputs.

Cache misses must remain valid cold installations. mise's
[download directory](https://mise.jdx.dev/directories.html) is not a supported
download cache; use its documented
[CI caching contract](https://mise.jdx.dev/continuous-integration.html#caching)
instead of a custom downloader or retry loop. An incompatible runner image or
architecture requires a separate cache namespace. The current Linux boundary
uses a dedicated runner; do not reuse that runner identity across architectures.

### Source checks

The [quality and platform evidence policy](docs/governance/change-and-release-policy.md#quality-and-platform-evidence)
defines supported measurements and admission boundaries; native tool
configurations own executable rules. Package observation and statement
coverage are distinct: a proven zero denominator is not a percentage.

The source secret check uses Gitleaks on current regular files selected by
Git: tracked files, including ignored tracked paths, plus nonignored untracked
files. Deleted files and symlink targets are outside this current-file scan;
Git history and ignored build output are different evidence domains. The
existing CI entrypoint copies that inventory into one private, path-preserving
directory under `build/tmp/`, runs Gitleaks once with the repository policy and
redaction, then removes the copy on success or failure. The input files remain
unchanged. Run it alone with
`mise exec --locked -- go run ./tools/ci check-secrets .`.

### Native upgrade acceptance

`mise run native` verifies source behavior, then builds the archives through the
same GoReleaser owner as publication and runs the current host's installation,
upgrade, invalid-successor rejection, rollback, and uninstall acceptance.
Construction and acceptance share one temporary scope, reclaimed on success or
failure. This does not publish or sign a release and needs no signing credential.

To qualify the complete repository toolchain on the current platform, run:

```bash
mise exec --locked -- go run ./tools/ci native --full-quality
```

This replaces the first Go-only check with the existing complete quality graph,
then runs the same platform-selected tests and packaged lifecycle. It does not
duplicate the Go check or require publication credentials. GitHub's manual
**Verify** input `full_quality` selects this path on the selected native platforms;
GitLab accepts `AIGW_FULL_NATIVE_QUALITY=true` for its available native jobs.
Ordinary review jobs retain their smaller native path and separate quality job.
Keep source-signature admission in that quality job; native tool qualification
does not replace it or infer signer authority from an inherited variable.

For a targeted manual qualification, GitHub's `native_platform` accepts `all`
(the default), `darwin`, `linux` or `windows`. GitLab's manual/API pipelines use
`AIGW_NATIVE_PLATFORM` with the same values; omitted or empty means all available
native jobs. Forge runner availability still applies. Quality runs in either
case. Combine the selection with `full_quality` and `refresh_locks` on GitHub,
or `AIGW_FULL_NATIVE_QUALITY=true` and `AIGW_REFRESH_LOCKS=true` on GitLab, to
qualify one platform's complete toolchain and lock resolution without rerunning
unrelated native jobs. Review, accepted-branch push and tag admission retain
their full native set regardless of this manual selector. A targeted result
proves only the selected platform, never complete CI or release readiness.
GitHub [manual workflow checks](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks#checks-from-some-workflow-jobs-are-not-evaluated)
do not replace the required pull-request checks; retain the review run separately.

The default native suite builds a synthetic predecessor and candidate through
the same GoReleaser archive construction used for releases. On macOS, its
test-owned certificate files exercise archive signing without importing an
identity, changing host trust or reading production credentials. Signing and
credential authorization remain separate boundaries; see the
[credential decision](docs/decisions/dr-0011-single-portable-token-backend.md#product-reader-and-migration-boundary).
An explicitly supplied archive is consumed unchanged and is never
replaced by a source build. Private fixture signing does not establish
authorization to credentials created by a historical released executable.
It tests portable update mechanics, not compatibility with a historical
release. An explicit published baseline adds a separate journey using a
manifest the released predecessor can actually read. That journey installs the
exact candidate, previews and applies its schema migration, synchronizes,
restores the predecessor's exact configuration, rolls back the program, and
repeats the forward transition. The current-schema package and real-client
journeys retain their own predecessor fixture. To exercise the published
transition, supply an extracted native binary from an independently verified
archive:

```bash
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/released/aigw \
  mise exec --locked -- go run ./tools/release accept-native
```

On Windows, set the same environment variable to the extracted `aigw.exe`.
The candidate uses the [canonical version](VERSION) and must be newer than the baseline.
An invalid explicit baseline fails rather than falling back to a fixture in
the published journey. Each path installs into a temporary home and compares
the installed binary bytes. The published transition uses an environment
credential and a stub client; the current-schema path also verifies deferred
activation and native projections. Neither modifies the operator's installation.

Ordinary Go tests use provider doubles and do not touch the host credential
store. On a disposable native test host, `AIGW_VERIFY_SYSTEM_KEYRING=1` exercises
the current-schema predecessor fixture and packaged candidate through the system
credential store. It verifies rotation, upgrade, rollback, re-upgrade, uninstall, reinstall,
retained helper credentials, and exact test-slot deletion. Each replacement
keeps the Adapter enabled and checks configuration bytes before `sync` can
change them. macOS additionally
requires `AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host`. This declaration
is an operator prerequisite, not proof that a machine is disposable; never set
it on the operator workstation. Linux requires a
real user bus and Secret Service for this path; the no-bus fallback is a separate
journey. Windows exercises
Credential Manager. An occupied test slot fails before mutation.
The installed AIGW helper, not the test executable, proves Token reads.
The fixture observes slot presence and performs exact cleanup; requiring it to
read the Token would add an unrelated reader-authorization prerequisite.

Keep daemon diagnostics separate from the product command's result. GNOME
Keyring 48.0 emits an already-registered-item warning when replacing an existing
credential. The upstream `CreateItem` implementation attempts item-created
registration even when it replaced the item; the same path remains in
[50.0](https://github.com/GNOME/gnome-keyring/blob/50.0/daemon/dbus/gkd-secret-objects.c).
A disposable upstream-only probe reproduces the warning with go-keyring 0.2.8:
the replacement is readable, the collection contains exactly one item, and
deletion leaves it empty. This is not evidence of an AIGW duplicate credential.
Retain the warning and exact backend version in acceptance evidence; do not
silence it, delete before replacement, or claim warning-free qualification.

### Existing candidate acceptance

To qualify a complete signed release matrix without rebuilding it, supply the
[public trust inputs](#hosted-release-verification), set `CI_COMMIT_TAG` to its
exact signed tag, and run:

```bash
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/released/aigw \
  mise exec --locked -- go run ./tools/release accept-native \
  --artifacts /absolute/path/to/published/matrix
```

The command verifies signatures, provenance and complete inventory before
executing anything from the matrix. It copies only the native archive and
checksum file into owned scratch, extracts through the product's verified
archive reader, and runs both the current-schema lifecycle and the separate
published-predecessor migration when a baseline is supplied. Source artifacts remain
unchanged; success and failure both reclaim scratch. `--clients` adds the same
current-schema real-client journey described below. The same candidate also runs the reviewed
team manifest through import without Tokens or clients, each Account becoming
available independently, deferred client synchronization, stable repeated sync,
credential-helper execution and uninstall. Client discovery uses fixtures here;
`--clients` remains the separate real-client proof. Omit `--artifacts` for
source-built native acceptance; the two inputs share extraction and tests.

The packaged suite also verifies configuration-aware rollback admission. It
uses a real predecessor's inability to read newly imported configuration when
present, otherwise injects an unsupported field. Rejection must preserve both
programs and configuration bytes. After an explicit compatible restoration,
the predecessor must export that configuration successfully, and re-upgrade and
uninstall must still work. Historical inability is observed, not inferred from
version strings; the current fixture alone does not prove historical support.

For a previously verified unsigned source-build candidate, the lower-level
tracked test also accepts an extracted native directory:

```bash
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/released/aigw \
AIGW_ACCEPTANCE_RELEASE=/absolute/path/to/verified/candidate \
  mise exec --locked -- go test ./tools/release \
  -run '^TestNativePublishedPredecessorJourney$' -count=1 -v
```

The candidate directory contains its native archive, `checksums.txt`, and the
archive's extracted native directory.
The suite consumes those exact bytes and never downloads or rebuilds them.
Record archive checksums and signature verification alongside the test result.
Real-client invocation remains a separate release requirement.

Inspect the operator's existing installation with its installed executable,
not an extracted candidate. Credential-helper projections bind an exact AIGW
path: a candidate run from another location expects its own path and can report
projection drift even when the installed helper remains correct. Do not run
`sync` to make this inspection green. Use the isolated lifecycle journey above
to validate the candidate at its intended installation path while preserving
the operator's configuration and credentials.

### Real-client acceptance

The tracked real-client journey reuses the same native installation and
replacement fixtures, with a controlled authenticated streaming endpoint:

```bash
AIGW_ACCEPTANCE_RELEASE=/absolute/path/to/verified/candidate \
AIGW_ACCEPTANCE_CODEX=/absolute/path/to/codex \
AIGW_ACCEPTANCE_CLAUDE=/absolute/path/to/claude \
AIGW_ACCEPTANCE_HERMES=/absolute/path/to/hermes \
AIGW_ACCEPTANCE_CLIENT_PATH=/usr/bin:/bin \
  mise exec --locked -- go test -tags=client_acceptance ./tools/release \
  -run '^TestNativeClientJourney$' -count=1 -v
```

Set equivalent environment variables on Windows, using native executable
paths and a semicolon-separated client tool path. Supply complete client
distributions and only their required companion tools. The test requires the
candidate and explicit client inputs and never substitutes a client stub or downloads software. It
uses synthetic environment credentials and temporary client homes, not the
operator's accounts or native credential store. It consumes the reviewed
`manifests/team.toml`, preserving Routes and recommendations while directing
Account endpoints to the isolated server. The server requires the recommended
model, configured effort and streaming protocol; a different model cannot
silently satisfy acceptance. The admitted clients execute at the current-schema
predecessor fixture, exact candidate, rollback and re-upgrade; uninstall preserves
post-setup authentication presence and bytes, plus user files. An absent
`auth.json` stays absent; an existing empty file is distinct from absence. The
verification fixture observes authentication storage without rewriting it.
Logs identify the exact client and artifact bytes. This proves the selected
clients' configuration and protocol integration, not live Provider
availability, model reasoning quality or an untested client version.

The build tag makes real-client execution an explicit acceptance operation,
not an optional test that reports success when clients are missing. Ordinary
quality checks still lint the tagged source and execute its input, streaming and user-file-preservation
contract tests without real clients. Both Forge quality jobs consume that same
existing command sequence.

To build one candidate and run both native lifecycle and real-client acceptance,
provide the same client and predecessor inputs and run
`mise exec --locked -- go run ./tools/release accept-native --clients`.
On macOS this explicit build requires the
[native release signing inputs](docs/decisions/dr-0011-single-portable-token-backend.md#release-identity-and-credential-authorization).
The release owner supplies the candidate directory, retains it for both tests,
stops on the first failure and cleans it afterward. No second build is needed.

#### Hosted Windows qualification

The existing GitHub **Verify** workflow exposes `windows_clients` for an explicit
manual qualification with `baseline_tag`. It provisions pinned official native
Codex and Claude packages using npm with install scripts disabled, verifies
registry signatures and resolves their complete native layouts before running
the same command. This path requires Git Bash and records executable hashes;
its temporary clients are removed afterward. It does not install anything on
the operator's workstation. GitLab projects the same native command onto its
local Windows runner through the required `AIGW_GITLAB_WINDOWS_RUNNER_TAG` CI
variable. GitHub remains exclusively GitHub-hosted; it exposes no self-hosted
runner selector or fallback.

Set `candidate_tag` with `baseline_tag` to consume a published signed matrix
instead of reconstructing its successor. Each native job downloads from its own
GitHub peer, supplies the public trust inputs, and runs `accept-native
--artifacts`; with `windows_clients`, the real Windows clients consume those
same candidate bytes. Run the workflow at the candidate tag's source revision.
This is native execution evidence, separate from peer-local asset verification.

#### Disposable Linux clients

For disposable Linux real-client acceptance, prepare the complete official
client distribution, including its companion executables. Preserve each AIGW
archive's canonical filename: the updater checks its target platform as well
as its bytes. Before running a journey, verify every input hash and its
readability as the actual test user; a successful copy is not that proof.
Secret Service also requires a registered user identity and a working user bus,
not just an unassigned numeric UID. In the disposable user's session, initialize
the empty test keyring with a newline on stdin, not immediate EOF, and verify
the login collection exists and is unlocked through its native D-Bus property
before invoking AIGW. Credential rotation also validates the endpoint: isolated
acceptance must provide its controlled loopback response, not an unreachable
placeholder. Provision fail-fast, then run clients without privileges or
capabilities. These test prerequisites never authorize host Keychain access.

Before the native suite, require `getent passwd "$(id -u)"`, an owned checkout
and Git directory, a writable private `HOME`, successful `git status`, and
disposable SSH key generation as that exact user. Recreate an incorrectly owned
fixture rather than change Git's trust policy or recursively relax permissions.
Native gh/glab version probes use test-owned configuration directories.

Use Docker's `--init` for disposable client containers so exited descendants
are reaped.
Copy inputs into live tmpfs through `docker exec -i` and a portable tar stream;
omit host extended attributes and the mount root's ownership metadata. Verify
input hashes as the actual test user. Retrieve results through the same live
mount and compare each file's hash before stopping the container; `docker cp`
may not see runtime tmpfs. Teardown must observe the test user's process set,
then remove the exact container and its temporary mounts.

Codex requires working user namespaces and bubblewrap for its Linux sandbox.
Probe those capabilities before acceptance rather than disabling the client's
sandbox to silence diagnostics. The measured disposable container has no
network or host mounts; its outer seccomp policy admits user namespaces, while
the client runs with no Linux capabilities and no-new-privileges. This is a
test-host constraint, not an AIGW installation requirement or a recommendation
to relax an operator's Docker security policy.

The lifecycle executes the projected credential helper through the native shell,
not a parsed copy of its arguments. Upgrade, rollback and re-upgrade keep the
Adapter enabled; configuration must remain byte-identical before the active
program synchronizes. Check the active executable, credential and readiness
after each transition. The predecessor must accept the fixture's configuration
and manifest schemas; an older-schema baseline needs its own reviewed migration
journey, not a disabled integration that hides incompatibility.

### Historical release qualification

GitHub's existing **Verify** workflow also accepts an optional `baseline_tag`
for manual historical-release verification. Select the candidate ref, its
exclusive `commit_base`, and a published predecessor tag. Each native job
downloads that peer's matching archive with GitHub CLI, verifies the published
SHA-256 checksum, and passes the extracted executable to the same package
lifecycle owner. The temporary download scope is removed on success or failure.
Windows always enables its native Credential Manager journey. Set
`macos_keychain=true` only on a disposable GitHub macOS runner to exercise the
same retained Keychain item across predecessor, candidate, rollback and
re-upgrade. Linux's hosted job proves the environment-credential lifecycle;
native Secret Service acceptance requires the separately provisioned test
environment described above.
An omitted tag retains the ordinary offline fixture-based journey. This checks
historical upgrade compatibility, not a release signature: older releases may
not include detached signatures. Local and GitLab operators can supply the same
explicit baseline path from their independently verified release artifacts;
neither peer depends on the other.

## Release and metadata

### Signed artifact builds

`mise run release` requires a clean source checkout and
`AIGW_RELEASE_SIGNING_KEY`, the path to the explicitly selected SSH signing key.
An agent-backed public-key path works when the matching private key is already
available through `SSH_AUTH_SOCK`; the public key itself does not sign. This
capability is separate from GitLab or GitHub transport access.

For a POSIX shell with an already available signing agent:

```bash
export AIGW_RELEASE_SIGNING_KEY=/absolute/path/to/release-signing-key.pub
export SSH_ASKPASS_REQUIRE=never
ssh-add -T "$AIGW_RELEASE_SIGNING_KEY"
mise run release
```

In PowerShell, set the same process environment variables using `$env:` before
running the same commands. The build does not install a key or start an agent.
If the key is unavailable, provision it through the host's credential owner;
do not disable signing or add an interactive password fallback to CI.

The build signs `checksums.txt` in the `aigw-release` namespace. Git's `git`
namespace and its allowed-signers entries do not authorize artifact signatures.
Verify the manifest against an independently approved public key and principal
whose allowed-signers entry admits `aigw-release`:

```bash
ssh-keygen -Y verify -f /absolute/path/to/release-allowed-signers \
  -I '<approved release principal>' -n aigw-release \
  -s dist/checksums.txt.sig < dist/checksums.txt
mise exec --locked -- go run ./tools/release validate-artifacts dist "$(cat VERSION)"
```

The first command establishes signer trust for the manifest; the second checks
matrix membership, file digests and the signature envelope. The validator alone
does not establish that the signer was authorized. Do not derive a trusted key
from the downloaded signature or confuse a successful local build with hosted
Release publication.

All three network entrypoints (`publish-github`, `upload-gitlab`, and
`publish-gitlab`) require `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE` and
`AIGW_RELEASE_ARTIFACT_SIGNER`. Supply an independently approved allowed-signers
file and its release principal; neither the Git author email nor a downloaded
signature selects this trust. Each entrypoint verifies the complete matrix and
its authorized `aigw-release` signature before its first network request.
GitLab uploads only the declared matrix inventory, not arbitrary files found
in the output directory. Verification does not access a private key or prompt
for a password.

GitLab publication accepts exactly one native credential: `GITLAB_TOKEN` for
an operator's personal, project or group access token, or `CI_JOB_TOKEN` for a
running CI job. These use GitLab's [access-token and job-token headers](https://docs.gitlab.com/api/rest/authentication/)
respectively. Both set is an ambiguous authority and fails before network
access; an absent token is not permission for anonymous publication. The same
selection applies to upload, release metadata and same-origin asset readback.
Cross-origin redirects receive neither credential. Supply tokens through the
approved process environment, never command arguments, URLs or tracked files.

Local release execution uses the same commands and explicit `CI_API_V4_URL`,
`CI_PROJECT_ID` and `CI_COMMIT_TAG` identity inputs; their names do not require
a CI runner. Building and signing locally does not require copying a private
key to either Forge. HTTPS or an approved protected transport remains an
operator prerequisite; a successful local fixture does not prove remote write
permission or publication.

Run publication from the product repository containing the selected
`CI_COMMIT_TAG`, and supply `AIGW_RELEASE_ALLOWED_SIGNERS_FILE` for Git source
trust. Before network access, each entrypoint resolves that local tag once,
verifies the tag and its exact commit, and reads the [canonical version](VERSION), lockfiles and the
toolchain from that immutable Git object, not the mutable checkout. Git replace
objects do not participate. Artifact provenance must equal the canonical
statement generated from those inputs and the actual artifact subjects.
Construction and verification share the same statement producer: no second
schema, parallel field mapping or independently maintained artifact list.

This establishes consistency between signed artifacts, provenance and selected
source. It does not establish an independently trusted build environment,
native runtime acceptance, remote tag equality or hosted observation. Those remain separate release obligations. Keep the artifact
directory exclusively owned and immutable throughout verification and upload.

### One signed matrix, independent publication

The operator builds and signs one release matrix from the accepted tagged
source. The existing `build-ci` command performs two builds and compares every
artifact before returning the result; its historical name does not require a
CI runner. Keep signing capability on the approved build host. Both peers
receive the same immutable files, including the original signature; neither
peer rebuilds, re-signs, or downloads from the other.

After release readiness, source acceptance and the exact signed tag are proven:

```bash
export CI_COMMIT_TAG="v$(cat VERSION)"
mise exec --locked -- go run ./tools/release build-ci build/release dist
mise exec --locked -- go run ./tools/release verify-artifacts dist
mise exec --locked -- go run ./tools/release publish-github dist
mise exec --locked -- go run ./tools/release upload-gitlab dist
mise exec --locked -- go run ./tools/release publish-gitlab dist
```

Run only the commands for selected peers, with their own previously admitted
transport credentials and identity inputs. A failed peer does not roll back
another peer or authorize reconstruction. The commands verify public trust and
tagged provenance before network access and compare uploaded bytes afterward.
Keep the identical matrix until every selected peer has passed readback.

### Hosted release verification

Hosted verification requires public trust, not a signing secret:

Configure three independent public trust inputs:

- **Git source trust:** `AIGW_RELEASE_ALLOWED_SIGNERS`.
- **Artifact trust:** `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS`.
- **Artifact principal:** `AIGW_RELEASE_ARTIFACT_SIGNER`.

GitHub uses repository variables for all three. GitLab uses file-type variables
for the two signer lists and an ordinary variable for the principal. The
workflow materializes or selects these files for the same verifier; neither
peer receives a private signing key.

Download authentication is separate: GitHub uses its read-only job token;
GitLab uses native CI job-token auto-login.

Tag push runs source and native checks. After all release assets are published,
dispatch GitHub's **Release** workflow with the exact `tag` input, and dispatch
a GitLab pipeline on that tag through the API or UI. These are explicit
post-publication observations: release-record creation can precede the final
asset upload, so it must not race the verifier. No operator confirmation dialog
is required; the delivering agent can dispatch through the native CLIs.

Each peer's `release-assets` job downloads only its own published assets with
locked `gh` or `glab`, then runs the same `verify-artifacts` command.
That command verifies complete inventory, bytes, authorized artifact signature,
annotated tag, signed source and source-bound provenance. GitHub grants only
`contents: read`; GitLab uses [native CI auto-login](https://docs.gitlab.com/cli/authentication/).
Missing trust, assets or authentication fails verification. Tag source CI,
artifact observation, native execution of released bytes and installation are
separate acceptance obligations; a successful one cannot replace another.

### Source and publication identity

Use focused Conventional Commits. Keep the [release chronology](CHANGELOG.md) with `## [Unreleased]` as
its first release section, containing only changes after the latest tagged
release. Every published heading must map to an existing `v<semver>` tag and
its tag date; run `mise exec --locked -- go run ./tools/release validate-changelog`
before requesting review.
GitLab **Project Name** is `AIGW CLI`; stable clone **Path** is `aigw-cli`. Do
not change external paths as a display-name cleanup.

Local Git owns one signed product commit and annotated tag. GitLab and GitHub
are equivalent, independent, optional publication peers that receive those
exact objects. Product signing and trust use `AIGW_RELEASE_AUTHOR_EMAIL` and
`AIGW_RELEASE_ALLOWED_SIGNERS_FILE`; peer transport authentication remains in
Git, SSH, or the protected host credential context. No peer-specific actor,
signing key, tag namespace, history replay, or tree-only equivalence is valid.

From a clean canonical checkout, `mise exec --locked -- go run ./tools/forge project` publishes
`main` atomically to peer `main` and `dev`, or one explicit `proposal/*` to its
matching ref. Candidate, work, and arbitrary branches are rejected. Ordinary
fast-forward and idempotent publication need no destructive option. A divergent
one-time cutover requires every exact observed remote tip and
`--force-with-lease`; restore protected-branch force push immediately after the
post-push observation. See [Forge Operations](docs/operations/forge-operations.md).

Protected CI supplies `AIGW_RELEASE_AUTHOR_EMAIL`,
`AIGW_RELEASE_ALLOWED_SIGNERS`, and the generated
`AIGW_RELEASE_ALLOWED_SIGNERS_FILE`. GitLab additionally owns
`CI_API_V4_URL`, `CI_PROJECT_ID`, `CI_COMMIT_TAG`, and `CI_JOB_TOKEN`; GitHub
owns `GITHUB_API_URL`, `GITHUB_REPOSITORY`, `GITHUB_TOKEN`, and `GH_TOKEN`.
These are execution inputs, never product defaults or repository identity.

Verify peer state directly with current `git ls-remote` observations. A branch
or tag is synchronized only when its full OID equals the local product object.
Hosted Release records and artifact bytes retain separate verification gates.

## Merge closeout

Follow [branch and worktree closeout](docs/governance/change-and-release-policy.md#branch-and-worktree-closeout).
Delete the exact merged proposal promptly; do not retain it merely because
release promotion is pending. Verify the peer's actual source-branch deletion
instead of assuming its project settings performed it. ETHOS owns local lane
retirement; product installation and other peers have separate evidence.
