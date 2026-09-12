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

| Output                                                | Owner and lifetime                                                                                                                         |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Build intermediates and release candidates            | Ignored `build/` and `dist/`; remove when superseded or published                                                                          |
| Raw verification output needed for an open decision   | Ignored `build/verification/<source-commit>/`; retain only until that decision is resolved and its required evidence has a durable owner   |
| Temporary downloads, extraction, and analyzer scratch | One private hidden directory per operation under `build/tmp/`, passed as `TMPDIR`; reclaim on success, failure, or interruption            |
| Dependency caches                                     | The package manager's cache; shared only where the tool supports it                                                                        |
| ETHOS evidence and coordination                       | The current command's native artifact reference; ETHOS owns its Git-common-dir storage and retention                                       |
| Published evidence                                    | The exact source revision's CI artifacts and release assets; local-only work retains required native evidence without depending on a Forge |

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
mise exec --locked -- go run ./tools/forge commits --email '<product author email>' --allowed-signers '<path>'
mise exec --locked -- go run ./tools/forge tags --allowed-signers '<path>'
```

## Projection changes

`aigw sync --dry-run --json` is a read-only planning surface. It may resolve
configuration but must not bind credentials, restart a client, modify a Codex
session, or write config/sidecar state. `aigw sync` prepares every configured
Codex target before its first write and rolls every target back if a commit
fails.

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
session, one deterministic synchronized target, and the selected Profile. A
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

`mise.toml` and `mise.lock` own language runtimes and standalone tools.
`package.json` and `package-lock.json` own OpenSpec, Prettier, markdownlint, and
their complete npm dependency graph. Use `mise run check` for the complete
source gate, `mise run native` for current-host acceptance, and
[`mise run release`](#signed-artifact-builds) for a signed, deterministic
non-publishing build. The tasks delegate to the
existing Go and npm owners rather than duplicating their behavior in shell
wrappers or a second task system.

`bootstrap` first runs `go mod tidy -diff`, then
`npm ci --include=dev --ignore-scripts`. Development dependencies are required
even when the caller sets `NODE_ENV=production` or npm's `omit=dev`; bootstrap
does not change either user setting.
The Go step prepares the complete source and dependency-test graph and rejects
lock drift without rewriting `go.mod` or `go.sum`. Merely compiling AIGW does
not populate every module needed for later dependency resolution. CI calls this
same task rather than maintaining a separate npm-only setup sequence.

The repository's native `mise` environment disables the persistent user Go
environment file and parent workspaces, and uses only the selected bundled
[Go toolchain](https://go.dev/doc/toolchain). `GOENV=off`, `GOWORK=off`, and
`GOTOOLCHAIN=local` apply to both `mise exec --locked --` and repository tasks;
they neither rewrite user settings nor download another compiler behind the
lock. An incompatible compiler fails explicitly. Deliberate process-level build
flags and platform targets remain separate inputs; these settings do not claim
to make an arbitrary shell hermetic. Bare `go` outside the managed environment
is not the repository verification entrypoint.

The npm tool commands live in `package.json` and run through the locked Node
runtime's `--run` entrypoint. The Go gate invokes those scripts rather than
interpreting platform-specific npm launchers. Formatting, Markdown lint, and
OpenSpec scripts address this checkout's installed package entrypoints;
missing dependencies fail and require `mise run bootstrap`, not a global-tool
fallback. For example, `mise exec --locked -- node --run markdown:check` runs
the same Markdown check used by the complete source gate.

[The native early configuration](.config/miserc.toml) stops mise's parent
configuration search before it reads tool declarations. It works from the
checkout root and nested directories; no shell wrapper or developer-specific
absolute path is required. Ordinary development retains explicitly selected
user settings, while CI uses the shared CUE configuration boundary below.

The shared CUE model assigns `MISE_CONFIG_DIR`, `MISE_GLOBAL_CONFIG_FILE` and
`MISE_SYSTEM_CONFIG_DIR` to the checkout's existing CI policy directory before
mise starts. The global file intentionally does not exist: it selects no user
configuration, not a file to create. Combined with the early parent boundary,
this excludes image, parent, user and system tool declarations. A private home
or `MISE_CONFIG_DIR` alone is insufficient. Disposable CI verification must
consume those projected values rather than approximate them; `mise config`
shows the resulting sources. The native regression executes all three Forge
projections from root and nested directories, preserves the owned setting and
proves foreign configurations remain unchanged and unselected.

An isolated verification checkout must preserve the measured source's commit,
actual branch role, remote metadata and required release tags. An exact commit
in detached HEAD is not equivalent to the original work branch for lifecycle
checks. Copy existing Git objects and metadata rather than inventing a
publication ref, synthesizing a tag or changing tracked files. Record those
inputs with the result; no verification checkout becomes an authoring lane.

### CI tool caches

[The CUE model](.config/ci/pipeline.cue) projects native tool caching to both
Forges. Every job still runs `mise install --locked`; cache presence is neither
verification evidence nor permission to skip a gate. `mise run bootstrap`
prepares Go dependencies and rebuilds Node packages in that checkout.

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
every declaration in `mise.toml`.

OSV Scanner uses mise's native Go backend so its integrated call analysis can
read the repository's Go language version. Go module checksums authenticate the
source build; upstream binary SLSA attestations do not describe this locally
compiled executable. The native executable-identity test also checks its
compiler against the locked Go version. After changing Go, rebuild a cached
scanner with `mise install --force go:github.com/google/osv-scanner/v2/cmd/osv-scanner`.
Then rerun `mise run check`; do not disable call analysis to accept an old build.

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

The default native suite builds a synthetic predecessor from current source.
It tests portable update mechanics, not compatibility with a historical
release. To repeat only the packaged lifecycle against a historical release,
supply an extracted native binary from an independently verified archive:

```bash
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/released/aigw \
  mise exec --locked -- go run ./tools/release accept-native
```

On Windows, set the same environment variable to the extracted `aigw.exe`.
The candidate version comes from `VERSION` and must be newer than the baseline.
An invalid explicit baseline fails rather than falling back to a fixture.
Each run installs into a temporary home, uses an environment credential and a
stub client, verifies upgrade, rollback, re-upgrade and uninstall, and compares
the installed binary hashes. By default it does not modify the operator's
installation or use the host credential store.

On a disposable native test host, `AIGW_VERIFY_SYSTEM_KEYRING=1` also exercises
the selected predecessor and packaged candidate through the system credential
store. It verifies rotation, upgrade, rollback, helper execution, retained
credentials after uninstall, and exact test-slot deletion. macOS additionally
requires `AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host`; never set this on
the operator's workstation. Linux requires a real user bus and Secret Service
for this path; the no-bus fallback is a separate journey. Windows exercises
Credential Manager. An occupied test slot fails before mutation.

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

To test an already built candidate without rebuilding it:

```bash
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/released/aigw \
AIGW_ACCEPTANCE_RELEASE=/absolute/path/to/verified/candidate \
  mise exec --locked -- go test ./tools/release \
  -run '^TestNativeProductJourney$/portable_artifact_lifecycle$' -count=1 -v
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

The tracked real-client journey reuses the same native installation and
replacement fixtures, with a controlled authenticated streaming endpoint:

```bash
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/released/aigw \
AIGW_ACCEPTANCE_RELEASE=/absolute/path/to/verified/candidate \
AIGW_ACCEPTANCE_CODEX=/absolute/path/to/codex \
AIGW_ACCEPTANCE_CLAUDE=/absolute/path/to/claude \
AIGW_ACCEPTANCE_CLIENT_PATH=/usr/bin:/bin \
  mise exec --locked -- go test -tags=client_acceptance ./tools/release \
  -run '^TestNativeClientJourney$' -count=1 -v
```

Set equivalent environment variables on Windows, using native executable
paths and a semicolon-separated client tool path. Supply complete client
distributions and only their required companion tools. The test requires all
four absolute inputs and never substitutes a stub or downloads software. It
uses synthetic environment credentials and temporary client homes, not the
operator's accounts or native credential store. Both clients execute at the
published predecessor, candidate, rollback and re-upgrade; uninstall preserves
post-setup authentication presence and bytes, plus user files. An absent
`auth.json` stays absent; an existing empty file is distinct from absence. The
verification fixture observes authentication storage without rewriting it.
Logs identify the exact client and artifact
bytes. This proves the selected clients' integration, not live Provider
availability, model reasoning quality or an untested client version.

The build tag makes real-client execution an explicit acceptance operation,
not an optional test that reports success when clients are missing. Ordinary
quality checks still lint the tagged source and execute its input, streaming and user-file-preservation
contract tests without real clients. Both Forge quality jobs consume that same
existing command sequence.

To build one candidate and run both native lifecycle and real-client acceptance,
provide the same client and predecessor inputs and run
`mise exec --locked -- go run ./tools/release accept-native --clients`.
The release owner supplies the candidate directory, retains it for both tests,
stops on the first failure and cleans it afterward. No second build is needed.

The existing GitHub **Verify** workflow exposes `windows_clients` for an explicit
manual qualification with `baseline_tag`. It provisions pinned official native
Codex and Claude packages using npm with install scripts disabled, verifies
registry signatures and resolves their complete native layouts before running
the same command. This path requires Git Bash and records executable hashes;
its temporary clients are removed afterward. It does not install anything on
the operator's workstation. The shared command is also available to local and
GitLab runners that provision equivalent inputs; no second product verifier is
introduced for a Forge that lacks a Windows executor.

For disposable Linux real-client acceptance, prepare the complete official
client distribution, including its companion executables. Preserve each AIGW
archive's canonical filename: the updater checks its target platform as well
as its bytes. Before running a journey, verify every input hash and its
readability as the actual test user; a successful copy is not that proof.
Secret Service also requires a registered user identity and a working user bus,
not just an unassigned numeric UID. Provision fail-fast, then drop privileges
and capabilities before running clients.

Codex requires working user namespaces and bubblewrap for its Linux sandbox.
Probe those capabilities before acceptance rather than disabling the client's
sandbox to silence diagnostics. The measured disposable container has no
network or host mounts; its outer seccomp policy admits user namespaces, while
the client runs with no Linux capabilities and no-new-privileges. This is a
test-host constraint, not an AIGW installation requirement or a recommendation
to relax an operator's Docker security policy.

The lifecycle executes the projected credential helper through the native shell
rather than parsing a particular helper argument layout. Upgrade is followed
by the active program's explicit `sync`; rollback first withdraws the fixture's
enabled integration, then restores the program and synchronizes again. Both
paths check credentials and readiness. The predecessor must accept the fixture's configuration and
manifest schemas; an older-schema baseline requires its own reviewed migration
journey and cannot establish compatibility through this test alone.

GitHub's existing **Verify** workflow also accepts an optional `baseline_tag`
for manual historical-release verification. Select the candidate ref, its
exclusive `commit_base`, and a published predecessor tag. Each native job
downloads that peer's matching archive with GitHub CLI, verifies the published
SHA-256 checksum, and passes the extracted executable to the same package
lifecycle owner. The temporary download scope is removed on success or failure.
The macOS and Windows jobs also enable the native credential journey against
those exact predecessor and candidate bytes. Linux's hosted job proves the
environment-credential lifecycle; native Secret Service acceptance requires
the separately provisioned test environment described above.
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

Run publication from the product repository containing the selected
`CI_COMMIT_TAG`, and supply `AIGW_RELEASE_ALLOWED_SIGNERS_FILE` for Git source
trust. Before network access, each entrypoint resolves that local tag once,
verifies the tag and its exact commit, and reads `VERSION`, lockfiles and the
toolchain from that immutable Git object, not the mutable checkout. Git replace
objects do not participate. Artifact provenance must equal the canonical
statement generated from those inputs and the actual artifact subjects.
Construction and verification share the same statement producer: no second
schema, parallel field mapping or independently maintained artifact list.

This establishes consistency between signed artifacts, provenance and selected
source. It does not establish an independently trusted build environment,
native runtime acceptance, remote tag equality or hosted signing-key
provisioning. Those remain separate release obligations. Keep the artifact
directory exclusively owned and immutable throughout verification and upload.

### Hosted signing inputs

Signing credentials belong to the release execution environment, not a
developer's login session. A CI runner being online does not prove that it has
a usable signing key. Both peers must authorize the same product artifact key
and principal if their independently constructed signed matrices are to be
byte-identical. Neither peer signs commits or tags on behalf of the other.

| Input                       | GitHub                                                                                        | GitLab                                                               |
| --------------------------- | --------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Source trust                | `AIGW_RELEASE_ALLOWED_SIGNERS` variable, materialized by `ci trust-input`                     | `AIGW_RELEASE_ALLOWED_SIGNERS` file-type variable                    |
| Artifact trust              | `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS` variable, materialized by `ci trust-input --artifact` | Protected `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS` file-type variable |
| Artifact principal          | `AIGW_RELEASE_ARTIFACT_SIGNER` release-environment variable                                   | Protected `AIGW_RELEASE_ARTIFACT_SIGNER` variable                    |
| Artifact public key         | `AIGW_RELEASE_SIGNING_PUBLIC_KEY` release-environment variable                                | Included in the approved artifact trust file                         |
| Artifact signing capability | `AIGW_RELEASE_SIGNING_PRIVATE_KEY` release-environment secret, loaded into an ephemeral agent | Protected `AIGW_RELEASE_SIGNING_KEY` file-type variable              |

Use a dedicated unencrypted automation key, with access controlled by the Forge
secret store. Do not upload a personal key, register this key for Git transport,
reuse a host login agent, or print private material. Configure GitHub's
`release` environment to admit only the intended release tags before adding
secrets; the workflow's environment name alone creates no protection. GitLab
signing inputs must be restricted to protected release refs and unavailable to
proposal/MR jobs. Provisioning and access controls require direct hosted
verification; a workflow declaration is not proof that they exist.

GitHub uses the pinned `webfactory/ssh-agent` action for agent creation, key
loading and post-job cleanup; it runs only after source checks. The capability
check verifies the configured public key is usable by that agent before
construction. GitLab checks required trust files and noninteractive private-key
access before construction. Missing inputs stop the job rather than falling
back to personal credentials. Upload and release jobs need public trust and
transport credentials, not a signing key.

The same local build and publication commands remain available without either
Forge. Hosted setup is not required for local-only artifact construction.

### Source and publication identity

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
