# Contributing to AIGW CLI

## Scope

AIGW controls Accounts, credentials, Models, Routes, Client Bindings and guarded native
projections, not conversations, API traffic or external services. Start from the
[architecture boundary](docs/architecture/authority-and-projection-boundary.md) and current [OpenSpec](openspec/) requirement, not files or old code.

## License

Contributions use the repository's [MIT License](LICENSE).

## Working method

### Closing a repair

Identify the contract and owner; separate product prerequisites from implementation
assumptions. Reproduce the failure, retain a distinguishing regression and repair
the owner. Retire superseded mechanics; review source, tests, schemas and guidance.

For admission and hooked Git, set `ETHOS_ACTOR` to the current Lease holder and
`ETHOS_CHANGE` to the selected Change's logical ID. Derive fixture expectations
from actual consumers/locks; isolate configuration discovery. Run the changed
package's format, static and size checks before the native matrix.

On stable inputs, run focused checks before one complete gate. Qualify shipped
manifests through actual delivery, not fixtures. Native authorization tests retain
the deployed security format and effective boundary, not an API imitation in
temporary storage. Before full gates/benchmarks, execute each changed command on
its actual candidate and retained predecessor in the declared starting state;
case-table success does not prove first-time acceptance of configured state.

Because umask filters creation modes, set permission-sensitive modes afterward.
Test ordinary/restrictive masks in separate processes, not by changing a concurrent
test's global umask. Evidence restrictions apply only to the exact directory.

Native journeys report explicit candidate digests and reject substitute builds for
missing inputs. Preserve published-predecessor configuration and enabled Adapters
through upgrade/rollback. Use selected subtests' `testing.T` for source fixture
resolution, failures and cleanup; precompiled/native-store cases build no unselected fixtures.

Publish readiness atomically: only complete bytes make the final path ready; parsing retries cannot excuse partial publication.

Capture credential invocations before replacement; run them before sync/reload.
Native-store proof identifies both readers and preserves the original item. Live
cutover requires complete program/configuration rollback and original-caller
acceptance; local workarounds are not approved dependencies.

Record results in the existing OpenSpec task; remove contradicted guidance. Keep
source, packaged, installed, hosted and published acceptance distinct: notes,
rules and formatter success do not prove prevention.

### Bounded TDD journey

From the repository root, select Mise's native configuration discovery before
starting it. CI projects the same boundary from `.config/ci/pipeline.cue`:

```bash
export MISE_CONFIG_DIR="$PWD/.config/mise/external"
export MISE_GLOBAL_CONFIG_FILE="$MISE_CONFIG_DIR/config.toml"
export MISE_SYSTEM_CONFIG_DIR="$MISE_CONFIG_DIR"
export MISE_IGNORED_CONFIG_PATHS="$MISE_CONFIG_DIR"
```

These process-local inputs ignore one external configuration tier; no file or
directory is created there. Native project discovery and task working directories
stay unchanged, and operator settings are not edited. The project `miserc.toml` limits parent discovery but
cannot retroactively exclude global or system configuration read at startup.

1. Select the [requirement](openspec/) and semantic owner.
2. Confirm a distinguishing regression's intended RED.
3. Make the smallest complete repair; retire the replaced path.
4. Run focused GREEN and affected consumers, then `mise run check` once stable.
5. Verify the [introduced signed commit range](docs/operations/forge-operations.md#verify-local-objects); preserve native and delivery obligations.

Formatter-only work uses native format/structure checks, not invented behavioral
failures. Optional indexes aid navigation, not proof: check their exact-root
coverage and freshness, then verify uncertain or excluded inputs against source.

### Analyzer isolation

Read-only analysis may inspect `main`. Write-capable analysis uses an owned
non-`main` worktree and private `TMPDIR`. Before retirement, prove that its task
has handed off or terminated; visibility in an agent list is not liveness evidence.

### Output ownership and cleanup

| Resource                       | Owner and lifetime                                                                                            |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| Intermediates and candidates   | Ignored `build/`; remove when superseded or published                                                         |
| Pending verification           | `build/verification/<source-commit>/`; retain until its decision and durable evidence owner are settled       |
| Downloads and analyzer scratch | Private hidden directory under `build/tmp/`, passed as `TMPDIR`; reclaim on success, failure and interruption |
| Dependency caches              | Native package manager; share only through its supported cache contract                                       |
| ETHOS coordination and proof   | Current command's artifact reference; ETHOS owns Git-common-dir retention                                     |
| Published evidence             | Exact revision's CI artifacts and release assets; local-only work has no Forge dependency                     |

OS temporary storage is appropriate for short-lived native operations, not durable
handoffs. Cleanup belongs to the operation; after a crash remove only its exact
stopped resources. Preserve pending evidence before retiring its worktree.
For a private `GOMODCACHE`, run `go clean -modcache` with that exact cache selected;
do not change shared-cache permissions.

Keep source snapshots as `.go.txt` or archives. Ordinary `.go` files in ignored
`build/` still enter Go package discovery; a hidden operation directory such as
`build/tmp/.release-check/` does not. Retain tool-native output, not another
handwritten verdict or summary checksum. [`.serena/`](.gitignore) and other local
indexes remain ignored, disposable and outside product authority.

Create transported acceptance snapshots with native `git clone --no-hardlinks`.
Pin the exact source revision, retain its declared canonical remote as metadata,
and disable `core.untrackedCache` locally before checkout. Verify the source tree
and selected file hashes before mounting it read-only. This preserves native
build metadata without fetching a peer. A copied `UNTR` index extension or a
remote-less reconstruction can produce native-tool diagnostics; repair the
snapshot, not the destination's global Git policy or its logs.

## Projection changes

`aigw sync --dry-run --json` is credential-free planning: no lock, session edit,
client restart or projection write. Sync prepares all configured Codex targets
before writing and compensates in reverse order only where its own postimage
still matches. Preserve newer edits; see the [transaction contract](docs/decisions/dr-0006-transactional-client-projection.md).

Only Codex can establish that its projected model catalogue actually loaded.
After a catalogue or client-version change, use the tracked verifier:

```bash
mise exec --locked -- go run ./tools/codex/catalogcheck -model openai.gpt-6.1-sol
mise exec --locked -- go run ./tools/codex/catalogcheck -model gpt-6.1-sol-cdx -canonical-model gpt-6.1-sol
```

The optional `-canonical-model` must be the exact manifest Model ID for a
route-specific wire ID. The verifier uses an isolated client home, makes no
inference request, and compares the selected entry with the bundled canonical
metadata apart from `slug`. Retain client version, executable checksum and
entry digests. Exit 2 means a missing client or unproved input, not acceptance.

Live-client Route qualification instead uses `aigw verify --for codex` or
`aigw verify --for claude`; catalogue inspection and direct HTTP probes cannot
replace it. The commands require the synchronized selection and report the real
client version/digest without Token or response content. Claude runs in official
[bare mode](https://code.claude.com/docs/en/headless#start-faster-with-bare-mode),
without tools or session persistence. AIGW removes its managed Anthropic overrides
from the child; the projected helper retains its selected credential backend.
For unattended probes use isolated homes and explicit environment credentials.
A local protocol fixture proves client integration, not live Provider availability.

## Development and verification

```bash
mise run bootstrap
mise run check
mise run native
```

[Tool declaration](.config/mise/config.toml) and [lock](.config/mise/mise.lock) own runtimes and standalone
tools; [npm metadata](package.json) and [lock](package-lock.json) own OpenSpec and
text-quality dependencies. Mise tasks delegate to existing Go owners, not another
command plane. Bootstrap installs locked tools, checks `go mod tidy -diff`, and
runs `npm ci --include=dev --ignore-scripts`. Development dependencies remain
required despite caller `NODE_ENV=production` or `omit=dev`; user settings do not
change. Tidy prepares dependency-test inputs without rewriting [go.mod](go.mod) or [go.sum](go.sum).

Linux source acceptance requires `dbus-x11`, `gnome-keyring` and `libglib2.0-bin`.
It owns a private D-Bus session and disposable native store; the independent
Secret Service job qualifies released-product behavior rather than repeating
source tests. Git fixtures use compact, exactly cleaned private directories so
test names do not exhaust Windows path limits.

Both tasks and `mise exec --locked --` use `GOENV=off`, `GOWORK=off` and
`GOTOOLCHAIN=local`: user Go settings, parent workspaces and automatic compilers
cannot replace the repository [toolchain](https://go.dev/doc/toolchain). Process-level build targets remain
explicit inputs; this is not an arbitrary-shell sandbox. Bare `go` is not the
repository verification entrypoint.

Complete bootstrap before direct execution. A lock constrains installation, not
all executable lookup: [Mise may fall back to PATH](https://github.com/jdx/mise/issues/13649)
when auto-install is disabled and a tool is absent. A negative missing-tool test
uses `MISE_OFFLINE=1`; normal bootstrap remains online. The repository disables
Mise's optional versions host so locked installs need no extra version index.

Node and npm have independent pins. The native `npm:npm` alias takes precedence
over Node's bundled npm. Commit Mise-generated `.config/mise/locks/` sidecars with their
lock digest, without reformatting or manually translating fields. These are the
tool producer's dependency inputs, not another application manifest.

The [dependency policy](docs/governance/change-and-release-policy.md#dependency-maintenance)
owns freshness, age admission and reproducibility. Use native resolution for a
reviewed upgrade, then restore locked operation; preserve the startup boundary above.

Root placement follows native discovery, not a file-count target. Git, editor
and analyzer entrypoints, package manifests and paired locks, Forge projections,
and project entry documents keep their native homes. Tool policy uses semantic
`.config/` owners: [Mise](.config/mise/config.toml), [npm](.config/npm/npmrc), and
[format scope](.config/checks/prettier/prettierignore). Native Mise discovery
preserves the repository task directory; its lock and dependency graphs stay
together. Source and runtime resources remain with their consuming packages;
`build/` owns generated output. No root alias or compatibility symlink is added.

### Environment reconstruction

Ordinary source tests check declared tools and an empty-cache rejection, not a
second online bootstrap. Keep the provisioned Mise context while building probes;
only the AIGW child gets a disposable HOME and credential backend. Giving Mise a
new HOME can reinstall every tool.

Fresh-workspace acceptance starts with empty HOME/Mise/Go/npm caches, runs
bootstrap, verifies versions, checkout-local `node_modules` and unchanged locks,
and reclaims owned mutable state. An offline rerun proves cache reuse, not fresh
download availability. Do not rebuild OSV merely to match its embedded Go patch;
its pinned release and actual scan govern admission. CI scopes
`GODEBUG=http2client=0` only to tool installation after upstream HTTP/2 resets;
TLS, checksum verification and normal product transport remain unchanged.

### Native lock refresh

```bash
mise run dependencies:resolve
```

From a clean authorized checkout this refreshes the host platform twice and
compares all tool inputs with HEAD before and after each pass, including staged
changes. It checks metadata reproducibility, not version discovery.

For a reviewed version change, update the pin and use process-scoped
`MISE_LOCKED=0 mise lock <tool>` for preview/refresh. Commit native
`mise lock --sidecars --json` outputs; the producer retires unreferenced sidecars.
Use `mise lock --upgrade` only for a reviewed format migration and compare every
version and platform checksum. Installation alone cannot generate the full graph.

When upstream metadata needs authentication, scope
`MISE_GITHUB_CREDENTIAL_COMMAND` to the locked `gh auth token` command for that
resolver process. Never log/export its Token, initiate login or widen scopes.
GitHub manual `refresh_locks` provides its job token only to that step; GitLab
`AIGW_REFRESH_LOCKS=true` requires separately admitted `MISE_GITHUB_TOKEN` or a native credential-command input.
Both retain locks even on drift. Repository credentials do not confer upstream
access. Keep provenance checks and raw warnings; cache installation and online
lock resolution prove different properties.

### CI tool caches

[CUE](.config/ci/pipeline.cue) owns both peer projections; every job still installs
locked tools and bootstraps its checkout. GitHub's pinned action keys caches by
platform, image, Mise version, config/locks and job. GitLab Linux keeps `installs/`
and completion metadata together under `build/runtime/tool-cache/.mise/`, keyed
by directory roles, image digest, runner, job, branch and input hashes.

Preserve protected/review separation and complete-tool markers; an interrupted
installation cannot become ready by copying binaries alone. Cache copies exclude
credentials, user state, `node_modules`, product artifacts and proof. Saving on
failure preserves completed downloads but cannot change the job verdict.
Mise's [downloads directory](https://mise.jdx.dev/directories.html) is not a
supported cache; use its [native CI contract](https://mise.jdx.dev/continuous-integration.html#caching).
Cold misses and distinct image/architecture namespaces must work independently.

### Source checks

The [quality policy](docs/governance/change-and-release-policy.md#quality-and-platform-evidence)
separates measurements, command owners and acceptance. Zero native statements
are not a percentage. Gitleaks scans tracked regular files, including ignored
tracked paths, plus nonignored untracked files; deleted files, symlink targets,
ignored output and Git history have separate scopes. Its current-file owner
makes one private path-preserving copy, runs the selected redacted policy and
reclaims it without changing input bytes:

```bash
mise exec --locked -- go run ./tools/ci check-secrets .
```

### Native upgrade acceptance

`mise run native` runs source/resource tests in one canonical coverage invocation,
builds current-host archives through GoReleaser and exercises install, upgrade,
invalid-successor rejection, rollback and uninstall in owned temporary state.
It neither publishes nor needs signing credentials. Signed artifacts, published
predecessors, real clients and performance remain separate `accept-native` scopes.
Complete tool qualification replaces, rather than repeats, the Go-only check:

```bash
mise exec --locked -- go run ./tools/ci native --full-quality
```

Manual GitHub `native_platform` and GitLab `AIGW_NATIVE_PLATFORM` select
`all|darwin|linux|windows`; empty means all. Combine with `full_quality` /
`AIGW_FULL_NATIVE_QUALITY=true` and `refresh_locks` / `AIGW_REFRESH_LOCKS=true`
for targeted diagnostics. Source-native and full-quality runs share their locked
native tool closure; explicit prebuilt acceptance selects only artifact tools.

Lychee publishes a Windows x64 binary; Mise explicitly selects that
locked official asset for Windows ARM64, where Windows runs it through x64
emulation. Product archives retain their declared architecture.

Review, accepted
push and tag admission require quality and the complete native set. Scoped manual
artifact diagnostics may omit source quality; [manual runs do not substitute
for required PR checks](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks#checks-from-some-workflow-jobs-are-not-evaluated).
GitLab `AIGW_COMMIT_BASE` is an exclusive base; omitted means the selected
commit's first parent. Missing author/signer trust fails before other gates.

`accept-native --diagnostic-client <client>` runs only that supplied client's
retained predecessor journey using Go subtest selection. It requires admitted
prebuilt candidate and predecessor inputs, preserves source/artifact trust and
the original client budgets, and does not establish full product acceptance.
Manual peers expose `diagnostic_client` / `AIGW_NATIVE_DIAGNOSTIC_CLIENT`; leave
them empty for ordinary verification. GitHub also needs `native_clients=true`
to supply the host's clients. Review, accepted-push and tag jobs cannot use this
diagnostic scope instead of their full suites.

Default acceptance uses a synthetic current-schema predecessor. Test-owned
macOS signing fixtures neither import identities nor authorize historical
credentials. Explicit candidate bytes are never replaced by a source build.
A real published baseline adds a distinct retained-state journey: 0.1.0 requires
schema migration/rollback; 0.2.0 supports byte-preserving sync without migration.
An independently verified extracted portable binary can be supplied through
`AIGW_ACCEPTANCE_BASELINE`; Homebrew's installed executable is not that input.

Before tagging, admit the signed candidate and published predecessor matrices:

```bash
mise exec --locked -- go run ./tools/release accept-native \
  --artifacts /absolute/path/to/candidate-dist --candidate --clients \
  --baseline-artifacts /absolute/path/to/published-baseline --baseline-tag v0.3.1
```

Provide `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE`,
`AIGW_RELEASE_ARTIFACT_SIGNER` and `AIGW_RELEASE_ALLOWED_SIGNERS_FILE` independently.
`--candidate` defaults to the clean signed verifier HEAD. An updated verifier may
use `--candidate-source <full-commit-id>` to bind the same-version untagged product
to its exact signed commit; mutable refs, unsigned objects and mismatched
provenance fail before execution. A selected or same-version tag remains invalid
in candidate mode; after tagging omit it and select `CI_COMMIT_TAG`. Predecessor matrix inputs verify
complete signatures/tag/provenance before extraction and cannot be combined with
`AIGW_ACCEPTANCE_BASELINE`. Windows uses the extracted `aigw.exe`. Keep baseline
inputs scoped to `accept-native`, not the ordinary native suite. The candidate
must be newer; an invalid explicit baseline fails, never selects a fixture.

All journeys use temporary homes and compare actual bytes. Retain enabled
Adapters and configuration before sync. Disposable native-store opt-in is
`AIGW_VERIFY_SYSTEM_KEYRING=1`; macOS additionally requires
`AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host`, never on an operator host.
Linux needs a real user bus/Secret Service; Windows uses Credential Manager.
An occupied exact test slot fails before writes. The installed helper proves
reads; fixture metadata/cleanup neither grants access nor replaces them.

Use the Secret Service's native warning policy during D-Bus activation;
CI must preserve warnings without converting them into service termination.
Published AIGW 0.3.1 uses
go-keyring's `CreateItem(replace=true)` path, which emits a duplicate-registration
warning on the qualified GNOME backend;
[upstream retains that CreateItem path](https://github.com/GNOME/gnome-keyring/blob/50.0/daemon/dbus/gkd-secret-objects.c).
The successor searches first and replaces through `SetSecret`. Native succession
retains and reads the predecessor's original Token, verifies upgrade, rollback
and forward, then exercises successor rotation. It does not require the immutable
predecessor to repair its own rotation. Preserve original warning and failed
diagnostics; do not suppress them, delete before replacement or alter published
bytes to manufacture a pass.

### Tagged artifact acceptance

```bash
mise exec --locked -- go run ./tools/release accept-native \
  --artifacts /absolute/path/to/published/matrix \
  --baseline-artifacts /absolute/path/to/published-baseline --baseline-tag v0.3.1
```

Select public trust and exact `CI_COMMIT_TAG`. Alternatively use `--tag` and
`--baseline-tag` with `--peer github|gitlab --repository <repository>`; the chosen
native CLI owns downloads with no prompts, closed stdin and a two-minute deadline.
Local artifact paths never imply network access. The release owner verifies the
full matrix/source, copies native archive/checksums into scratch, extracts through
the product reader, executes both lifecycle scopes and reclaims scratch on failure
or success. Source artifacts remain unchanged. `--clients` requires explicit
client paths before construction/download starts.

The same bytes exercise the actual team manifest: no Tokens/clients, each Account
arriving independently, deferred sync, no-op preservation, helpers and uninstall.
Fixture discovery is not real-client proof. Rollback admission also requires the
retained program to export actual successor configuration in isolation. Rejection
preserves both programs and configuration; explicit compatible restoration then
permits re-upgrade/uninstall. A current-schema fixture alone is not historical proof.

### Real-client acceptance

The lower-level tagged test consumes the same explicit candidate and companion-tool
path, rather than searching the user's installation:

```bash
AIGW_ACCEPTANCE_RELEASE=/absolute/path/to/verified/native-directory \
AIGW_ACCEPTANCE_CODEX=/absolute/path/to/codex \
AIGW_ACCEPTANCE_CLAUDE=/absolute/path/to/claude \
AIGW_ACCEPTANCE_HERMES=/absolute/path/to/hermes \
AIGW_ACCEPTANCE_HERMES_HOME=/absolute/path/to/native/hermes-home \
AIGW_ACCEPTANCE_CLIENT_PATH=/usr/bin:/bin \
  mise exec --locked -- go test -tags=client_acceptance ./tools/release \
  -run '^TestNativeClientJourney$' -count=1 -v
```

Windows uses native executable paths and a semicolon-separated companion PATH.
Supply complete client distributions; a missing path is not a stub/download license.
On macOS and Linux, Hermes uses a disposable home outside its native data root.
It borrows committed dependencies without provisioning; boot records stay private.
Cleanup leaves native configuration, installation facts, boot records and launchers
unchanged. Windows verification refuses to start Hermes until native launcher
publication can be isolated.

The isolated Codex fixture explicitly enables the official Windows `unelevated`
sandbox, which uses a restricted token without administrator setup. The client
still runs in `read-only` mode; a permissions label without an active native
backend cannot qualify tool execution. This test-only setting neither changes
operator configuration nor chooses a production sandbox mode. Denied tool output
remains a failure even when Codex returns the final marker.

For focused retry-policy source acceptance, supply an explicit source-built
program, a portable predecessor fixture and the actual Codex distribution:

```bash
AIGW_ACCEPTANCE_RETRY_PROGRAM=/absolute/path/to/source-built/aigw \
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/retained/portable/aigw \
AIGW_ACCEPTANCE_CODEX=/absolute/path/to/codex \
AIGW_ACCEPTANCE_CLIENT_PATH=/usr/bin:/bin \
  mise exec --locked -- go test -tags=client_acceptance ./tools/release \
  -run '^TestNativeCodexConnectionRetriesSource$' -count=1 -timeout=2m -v
```

This exercises public preview, selection, cached predecessor credentials,
native budget admission, retained-reader rollback refusal, one unknown-outcome
request and native-default reset
in private homes with synthetic environment credentials. It does not qualify
package-manager cutover, release artifacts or Desktop behavior. The same case
runs as `codex-connection-retries` in the complete release journey.

The complete build-tagged journey requires all three declared executable paths:
`AIGW_ACCEPTANCE_CODEX`, `AIGW_ACCEPTANCE_CLAUDE` and `AIGW_ACCEPTANCE_HERMES`.
Hermes also requires `AIGW_ACCEPTANCE_HERMES_HOME` for its native installation.
Missing clients fail, not skip. See [Adapter admission](docs/governance/adapter-admission.md)
for independent Desktop qualification; CLI evidence cannot replace it.

A loopback protocol fixture consumes the shipped manifest and demands its exact
recommended model, effort, credentials and stream contract. Isolated homes and
synthetic environment Tokens replace operator state. With a published predecessor,
real clients run baseline → candidate → rollback → re-upgrade; without it Codex/
Claude use current-schema fixtures and Hermes proves first adoption only.

Before replacement the candidate stages its versioned helper; captured old/new
commands must remain callable. Verify retained authenticated inference before
active-program sync/check. The exact current candidate's per-client `verify`
observes every lifecycle state; it owns an isolated probe, not the retained
client or its settings. Original programs, byte checks, cached credential
commands, and independent native tool/session requests still qualify succession.
This does not prove the historical verifier works with a newer client:
0.3.3's Hermes verifier loses the installation binding in an arbitrary home.
Published 0.3.1 also cannot check the successor's projection until sync.
Uninstall preserves unowned settings and removes only the test installation.
No-token and actual Provider tests remain different observations.

Use the signed-candidate command above with `--clients`. Omitting artifacts
deliberately builds once; on macOS that explicit build needs
[release identity inputs](docs/decisions/dr-0011-single-portable-token-backend.md#consequences-and-boundary).
One retained candidate serves lifecycle and client tests; stop at first failure.
Quality jobs still run input/envelope/preservation regressions without real clients.
Homebrew's public-link replacement gap needs its own installed upgrade proof.

#### Hosted Windows qualification

GitLab review/protected jobs use separate registrations selected by literal CUE
tags; proposals stay unprotected. Disposable execution and peer-local evidence
remain required. GitHub uses hosted runners, not self-hosted fallback.

GitLab `AIGW_CANDIDATE_TAG` selects a published matrix;
`AIGW_CANDIDATE_ARTIFACTS` selects local pre-tag bytes. `AIGW_BASELINE_TAG`
selects the predecessor and `AIGW_NATIVE_CLIENTS=true` adds provisioned clients.
CUE forwards these after `ci native --`; source checks do not inherit them.
`AIGW_CANDIDATE_SOURCE` forwards the exact signed pre-tag product commit to
`--candidate-source` when a newer verifier consumes existing candidate bytes.

Explicit prebuilt inputs without full quality or lock refresh select only native
artifact tools and the existing release journey; they do not repeat source-wide
tests or npm bootstrap. Ordinary source/review jobs and full qualification retain
their complete quality and security graph. A partial artifact run cannot satisfy
the failed quality job or release readiness. GitHub dispatches use `Manual`
check names, including tag dispatches and single-platform runs; only review and
accepted push events report canonical required checks. Release evidence requires
the exact tag push and canonical job set. GitLab retains its complete MR graph
and native MR-pipeline admission, not a manually dispatched pipeline substitute.

GitLab downloads use its explicit project and Job Token, not sibling credentials.
GitHub manual `candidate_tag`/`baseline_tag` selects that peer's same bytes, with
`native_clients` for real-client execution. A reviewed verifier may have a
different SHA but must declare the candidate version; signed provenance still
binds execution to the selected source.

`ci native-clients supply` installs the clients its host declares, at the releases pinned in `.config/ci/pipeline.cue`, into one
job-owned directory: npm platform packages, and on Linux and macOS Hermes through its release's checksum-pinned installer.
A failed or interrupted supply removes it; `retire` removes only that directory, proves it absent and fails while a process still uses it.

#### Disposable Linux clients

Use Docker `--init`, a complete non-root account and owned tmpfs/output. Keep
the root filesystem read-only, drop capabilities and prohibit host mounts. Select
Mise at or above the repository minimum; the image must also supply `ssh-keygen`,
the native compiler and C runtime. Probe these inputs as the actual user before
starting product acceptance.

Docker tmpfs defaults to `noexec`. Explicitly allow execution only on private
Mise, work and test directories that contain tools, extracted product binaries or
Go test executables; retain `noexec` on artifact and trust input mounts. Check
actual mount flags and run the selected tools before a heavy gate. Synthetic
credentials never authorize host-store access.

Transfer inputs/results through `docker exec -i`. On macOS, create portable tar
streams with `--no-xattrs --no-acls --no-fflags --no-mac-metadata`; extract with
`--no-same-owner`. `docker cp` may miss live tmpfs. Verify hashes, process/resource
state and exact cleanup before stopping and removing only owned resources.

Codex requires working user namespaces and bubblewrap. Check both through the
actual client; containers are not interchangeable with native Linux hosts.
Never weaken an operator's security policy or silently remove the client's sandbox
to make acceptance pass. Use a test-owned Git repository where required rather
than global safe-directory changes. gh/glab probes use private config directories.

Retained-state acceptance executes projected helpers through the real native
shell and preserves configuration bytes before active-program sync. The baseline
must read the fixture's declared schema; older-schema inputs need the reviewed
migration journey, not disabled integration. Host, container, Provider and client
identities each have independent acceptance claims.

### Historical release qualification

GitHub **Verify** accepts `baseline_tag` and an optional `candidate_tag`, exact
candidate ref and exclusive `commit_base`. Jobs forward those inputs to
`accept-native`; that owner handles peer downloads, public trust, extraction and
cleanup. Forge Tokens never reach snapshot builders or native journey children;
client provisioning remains separate CI work.

Windows enables Credential Manager; disposable macOS opt-in `macos_keychain=true`
exercises a retained item through predecessor, rollback and re-upgrade. Linux's
ordinary hosted environment-backend proof is not Secret Service qualification.
Omitted tags select source-built current-schema acceptance. Published baselines
need complete signed matrices/source provenance; unsigned archives are rejected.
Local and GitLab input matrices remain independently verifiable without GitHub.

## Release and metadata

### Signed artifact builds

`mise run release` requires clean source and `AIGW_RELEASE_SIGNING_KEY`, an
explicit key path. An agent-backed public-key path signs only when the matching
private key is already available through `SSH_AUTH_SOCK`; do not start an agent,
read/export a key or add password fallback. Test the exact capability first:

```bash
export AIGW_RELEASE_SIGNING_KEY=/absolute/path/to/release-signing-key.pub
export SSH_ASKPASS_REQUIRE=never
ssh-add -T "$AIGW_RELEASE_SIGNING_KEY"
mise run release
```

PowerShell uses the same process variables via `$env:`. Product Git transport,
Git signatures, artifact signatures and Apple publisher identity are distinct.
The build signs `checksums.txt` in `aigw-release`; Git's namespace alone does
not authorize this signature. Use independently approved public trust:

```bash
ARTIFACTS="build/artifacts/$(cat VERSION)-$(git rev-parse HEAD | cut -c1-8)"
ssh-keygen -Y verify -f /absolute/path/to/release-allowed-signers -I '<approved release principal>' \
  -n aigw-release -s "$ARTIFACTS/checksums.txt.sig" < "$ARTIFACTS/checksums.txt"
mise exec --locked -- go run ./tools/release validate-artifacts "$ARTIFACTS" "$(cat VERSION)"
```

The first verifies authority; the second validates complete inventory/digests and
signature envelope. Never derive trust from the downloaded signature. All network
publication entrypoints require `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS_FILE` and
`AIGW_RELEASE_ARTIFACT_SIGNER` before contacting a Forge.

GitLab uses exactly one of `GITLAB_TOKEN` or `CI_JOB_TOKEN`; both or neither fail
before publication. [Native headers](https://docs.gitlab.com/api/rest/authentication/)
apply to uploads and same-origin readback, never cross-origin redirects. Tokens
stay in approved process environments, not URLs, arguments or tracked files.
Local execution uses explicit `CI_API_V4_URL`, `CI_PROJECT_ID`, `CI_COMMIT_TAG`
and source trust; these names do not require a CI Runner. Transport permission
and publication need actual observations, not a successful build.

Before upload, the owner resolves the tag once, verifies source and tree, requires
matching version/epoch, and compares signed provenance with those inputs and actual
subjects through one producer. This is consistency proof, not independent build
trust or installation. Keep the artifact directory immutable and exclusively owned.

### One signed matrix, independent publication

Build/sign once, then publish those identical bytes to every selected peer.
`build-ci` compares two reproducible construction passes; its historical name does
not require a Runner. Final Developer ID/timestamped bytes are separately verified
rather than claimed byte-identical. [Forge Operations](docs/operations/forge-operations.md)
owns tag publication and the native Apple submission/verification procedure.
Never rebuild, rewrite history or re-sign separately at each peer.

### Hosted release verification

CI materializes public source/artifact trust from `AIGW_RELEASE_ALLOWED_SIGNERS`
and `AIGW_RELEASE_ARTIFACT_ALLOWED_SIGNERS` into their corresponding `_FILE` inputs.
GitLab API/project/tag/Job Token and GitHub `GITHUB_API_URL`, `GITHUB_REPOSITORY`, `GITHUB_TOKEN` and `GH_TOKEN`
remain peer execution inputs, not product defaults or shared credential authority.
Use [Forge Operations](docs/operations/forge-operations.md) for publication commands.

After complete uploads, dispatch each selected peer's **Release** on the exact
tag; verification must not race assets. Each `release-assets` job uses locked
gh/glab and that peer's read-only/native authentication to run the same complete
signature/source/provenance verifier. Missing trust, assets or authentication
fail. Source CI, asset readback, [native released-byte execution](docs/operations/forge-operations.md#verify-published-bytes-on-a-native-host)
and installation remain separate acceptance boundaries.

### Source and publication identity

Use focused Conventional Commits and validate [chronology](CHANGELOG.md):
`mise exec --locked -- go run ./tools/release validate-changelog`.
`## Unreleased` contains only changes after the latest published tag; historical
headings require their exact signed tag/date. GitLab display name `AIGW CLI`
does not authorize changing the stable clone path `aigw-cli`.

[Branch publication](docs/operations/forge-operations.md#publish-a-branch) owns
exact-object admission, remote leases and authorized divergent cutover.
Source trust uses `AIGW_RELEASE_AUTHOR_EMAIL` / `AIGW_RELEASE_ALLOWED_SIGNERS_FILE`;
Forge transport remains independently authorized. History maps, peer-specific
actors and tree-only equivalence do not establish object identity.

## Merge closeout

Follow [branch and worktree closeout](docs/governance/change-and-release-policy.md#branch-and-worktree-closeout).
Delete the exact merged proposal promptly even while release promotion waits;
verify actual peer absence. ETHOS owns local retirement, not installation or
another peer's completion.
