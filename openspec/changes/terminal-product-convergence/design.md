# Design

## Context

See [the proposal](proposal.md). Current accepted source, Work Lane authority,
and foreign ownership are resolved through ETHOS at execution time, not copied
into this design. Any working installed predecessor remains protected. Current source
already has one Account/Route/Client Binding model, an environment backend,
native client adapters, a CUE CI projection, and an AIGW-owned credential copy.
The terminal work is to make those owners sufficient, not add another routing
service, configuration hierarchy, or status ledger.

## Goals / Non-Goals

**Goal:** deliver and retire the existing AIGW convergence before the existing
provider-neutral Responses Proxy convergence. Acceptance covers these semantic
outcomes through the existing task owners, not another checklist:

- **Onboarding and readiness (2):** optional Accounts and clients, independent
  explicit bindings, late sync, ownership-preserving merge and compensation,
  and consistent setup, use, status, check, verify and doctor decisions.
- **Credential continuity (3):** one native or explicitly selected environment
  backend; precise Account and diagnostic item ownership; authentic retained
  predecessor reads, signed successor qualification, original-command survival,
  bounded package replacement and rollback without prompt loops.
- **Routes and clients (4):** current qualified provider wire IDs, protocols,
  capabilities and consistent catalogue grammar; ordered usable recommendations
  preserve explicit/manual choices. Each Codex, Claude and Hermes mode proves
  native model selection, authentication, tool use and continuation separately.
  Other adapters require a complete ownership and withdrawal contract.
- **Portable lifecycle (5):** actual macOS, Linux and Windows setup, upgrade,
  rollback, forward, uninstall and failure-cleanup journeys with exact candidate
  bytes, retained state and independently reconstructed locked environments.
- **Quality and performance (6):** strict proportionate all-format gates,
  narrow public types and errors, supply trust and measured predecessor budgets;
  modern syntax or tools serve meaning rather than add superficial machinery.
- **Forge admission (7):** one CUE graph generates symmetric optional peers;
  proposal/review, maintainer, dev/main and tag paths bind exact signed objects,
  required checks, runner containment and complete cold supply custody.
- **Repository meaning (8):** deep cohesive modules and tests, one authority per
  concern, lean English navigable docs/configs and rendered diagrams, qualified
  references, bounded decision-support research and consumer-based deletion.
- **Delivery (9 and Migration Plan):** one frozen accepted candidate, governed
  integration and official archive, SemVer/Changelog, signed immutable assets,
  native package-manager installation and exact retirement of owned residue.

**Completion:** accept the [task checklist](tasks.md) through its
[requirement and acceptance routing](#requirement-and-acceptance-routing), then
finish the [Migration Plan](#migration-plan): guarded integration, official
archive, publication, installed upgrade and rollback, and exact retirement.
Source checks, signed candidates, pipeline success, and task counts are distinct
intermediate evidence, not installed-product completion. A requested client mode
remains unqualified until its own native contract passes.

Finish AIGW before extending the existing Responses Proxy convergence; a live
service incident permits only bounded restoration before returning to that
order. Close one semantic acceptance gap at a time and reuse unchanged qualified
inputs. This work retains its current Goal, Change, lane, and sole task ledger.

Qualify a capability with its exact client mode, protocol, provider wire ID,
artifact and retained state. A previous failure is not a permanent exclusion;
newer acceptance supersedes it only within the measured scope. A reproducible
contradiction reopens the owning task before dependent delivery. Unsupported
native behavior remains an explicit limit, not an invented adapter or success.
Refresh repository-owned pins to the latest compatible stable releases before
the final input freeze; host upgrades do not alter locked project inputs.

Each closure uses its existing owner: focused RED/GREEN where behavior changes,
sibling-consumer reconciliation, exact staging, affected gates, evidence and
owned cleanup. Evidence references retain failed results and rollback consumers.
Two unsuccessful repairs trigger a new hypothesis before another expensive run.
Every active-hour checkpoint reports facts, evidence, unproved scope and next
action in the existing task context. Guidance is not enforced Agent behavior.
Bound each Agent/tool input, output and execution to its owning operation;
context pressure is not evidence of a new credential or transport defect.

**Non-goals:** request transport, transparent mid-request retries, external
Proxy lifecycle, Codex conversation or model-choice mutation, automatic browser
or Keychain authorization, a universal client config patcher, a new daemon,
or a second OpenSpec/ETHOS command plane.

## Execution Dependencies

This is dependency order, not a second progress ledger. The checklist owns
state; the routing table owns acceptance; the Migration Plan owns delivery.

| Order | Semantic closure                                                                                  | Existing task owners                            | Exit condition                                                                                                                             |
| ----- | ------------------------------------------------------------------------------------------------- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | Repair CI resource declarations; prove peer admission, runner containment and cold supply custody | 7.4, 7.5                                        | Declared caches/artifacts have real consumers; exact checks reject invalid admission; each selected peer remains independent.              |
| 2     | Consume the accepted ETHOS identity/reference successor                                           | 8.3.2                                           | The installed adopter rejects wrong-repository references and qualifies the authorized history repair; no private checker.                 |
| 3     | Freeze one candidate for retained-predecessor, native-client and performance acceptance           | 3.5, 4.5, 5.2, 6.6, 9.3                         | The same candidate passes required macOS/Linux/Windows, store, client-mode, upgrade, rollback, forward and cleanup journeys.               |
| 4     | Deliver and retire AIGW through the Migration Plan                                                | Migration Plan                                  | Exact signed integration, archive, publication, package-manager cutover and installed acceptance pass; owned obsolete consumers retire.    |
| 5     | Resume the existing provider-neutral Responses Proxy closure                                      | Existing Proxy Change and its authorized holder | Replay, transport, native lifecycle, repository quality, release and installed-product evidence satisfy that Change without AIGW coupling. |

Only the accepted-ETHOS-dependent scope waits for its successor. Advance
independent candidate preparation without competing host assays. A prose-only
commit needs fresh exact-HEAD governance, not rebuilt unchanged product bytes.

## Decisions

### 1. Keep semantic owners, remove competing decisions

`configuration` owns authored Accounts, canonical Models, Routes,
recommendations, explicit Client Bindings, and compatibility. `activation`
shall derive one credential-free decision from that configuration, currently
discoverable clients, and backend availability metadata. It returns separate
capability, selected, connected, projected, and verified states plus ordered
prerequisites. The CLI projects this same decision into setup, status, sync,
check, doctor, human output, and JSON; each command adds only its own effect or
observation. The next action must be executable now, or name its exact
prerequisite. An available recommendation may fill an unselected binding;
neither a recommendation nor an unrelated Account may replace an explicit
selection. Delete the local next-action choosers made redundant by this owner.

A Route's `Model` is the canonical AIGW Model ID; `UpstreamModel` is the
exact provider wire ID. Runtime preserves the existing wire-facing `Model`
field and carries the manifest's exact mapping separately as
`CanonicalModelID`. Codex may add a wire alias only by copying the exact
canonical entry from its bundled table and changing `slug`; it must not infer
a base from suffix similarity. A user-authored `model_catalog_json` remains
outside AIGW ownership.

An enabled Client Binding records intent, not native projection. A binding with
no recorded executable has a deferred projection prerequisite; a previously
recorded executable or target that fails native inspection is a repairable
failure instead. When every enabled projection is deferred, observational
commands must not probe a provider or suggest `aigw check` as activation.
Client-specific verification remains a separate, later claim.
Selected Account availability is observed through metadata only and cached in
the activation decision. When both its Token and projection are absent, the
decision retains both facts and offers Token recovery first; after recovery,
installation if needed and synchronization become the continuation. A ready
alternative Client Binding may proceed without waiting for an unrelated Token.

The existing manifest remains token-free team policy. Each Account Token is
optional until a selected operation requires it. Native keyring, guarded file,
and process environment are alternative backends with one recorded owner per
invocation; an unavailable backend fails explicitly and never silently falls
through to another. Client discovery never creates a missing client's files.
Manifest setup imports a reviewed declaration and projects the locally usable
client intersection without a provider request. This removes the inconsistent
online gate that previously rejected an offline import although `sync` could
project the same declaration. A connected Token and projection remain local
facts; `check` alone observes endpoint authentication and inference. Guided
single-route setup and Token rotation retain their explicit validation contract.

### 2. Separate declarative choices from live data-plane failures

The ordered primary/alternative recommendations already carry preference for
an unselected Client Binding. Add no second priority list. A provider not in
that list is manual-only for that client. Credentials and declared protocol
capabilities may determine which recommendation is usable at setup/sync time;
`aigw verify` and `aigw test` observe one explicit route without rewriting it.
AIGW cannot observe every native client's live request, so it must not promise
transparent failover or replay. If request-time routing becomes a product need,
qualify an optional data-plane service under its own authority and contract.

Models are admitted by protocol and capabilities rather than name-family
switches. The team catalogue is reviewed authored guidance, not proof of
current upstream availability. Catalogue maintenance compares upstream
metadata with bounded real inference, native client support, and exact model
IDs; it removes retired entries and preserves explicit local choices. GPT,
Claude, and other Provider families follow the same grammar, with supplier
variants only when their upstream contract truly differs.

Low-level manifest import retains local-only Routes by default. An operator may
name obsolete local Routes for retirement in the same guarded configuration and
client-projection transaction as the import. Retirement rejects a Route still
declared by the incoming manifest or selected by any Client Binding. Replaced
recommendations must remain valid; only Models left without any Route reference
and absent from the incoming manifest are removed. Account metadata and Tokens
are not retired by this operation.

### 3. Qualify a Client Adapter, do not infer support from a file

Each Adapter owns only its client's executable discovery, native config
projection, explicit model/protocol choice, credential contract, guarded merge,
rollback, withdrawal, and real-client tool-loop verification. Keep the existing
registry and adapters as the extension seam; do not branch core Account or
release logic on Provider names. Claude Code and Claude Desktop are separate
surfaces; Codex CLI and Desktop share their selected home without altering
conversation metadata. Hermes, pi, OpenCode, WorkBuddy, Qoder, and future
clients are assessed against the same admission checklist. Unsupported or
non-extensible modes are reported as such, not approximated by writing a
plausible config file.

OpenCode is a feasibility candidate, not an admitted Adapter. On 2026-09-28,
its installed 1.18.32 CLI listed an isolated custom model and completed a
synthetic-Token Chat Completions `read` tool loop without user state. This
disposable probe is not a tracked acceptance test. Its [config precedence](https://opencode.ai/docs/config/)
lets project and managed settings override a global projection; its
[custom Provider contract](https://opencode.ai/docs/providers/) selects different
SDKs for Chat Completions and Responses and supports environment-based API keys.
An AIGW-owned JSONC-preserving merge, exact withdrawal, effective-config check,
and credential-mode contract remain unproved. Do not add the Adapter merely
because a local endpoint responded. Pi, WorkBuddy, and Qoder likewise remain
unadmitted without an executable and real tool-loop evidence.

At signed source `e458271e`, the tracked macOS `TestNativeClientJourney` passed
with installed Claude Code, Codex, and Hermes executables, isolated client homes,
synthetic environment Tokens, and a local streaming Responses server. It
exercised Claude/Codex replacement and rollback, Codex general Routes and tool
loop, and Hermes's retained two-turn session. This is real-client configuration
and protocol evidence, not live Provider inference, Claude Desktop acceptance,
another operating system, or published-artifact qualification; task 4.5 remains
open.

### 4. Decide credential-command continuity before cutover

The accepted [credential decision](../../../docs/decisions/dr-0011-single-portable-token-backend.md)
rejects the independent `aigw-keychain` helper. The existing private copy is
the same AIGW executable, not another Token reader. DR-0011 now selects a
content-addressed direct path over fixed replacement and indirection after a
platform-contract comparison. Source implementation alone still does not admit
an installed cutover or establish native credential authorization.

The comparison and rejected alternatives are recorded in DR-0011. Native
qualification still tests original cached commands during update and rollback,
credential access without prompts, executable replacement, interruption, and
exact cleanup on macOS, Linux, and Windows. The
`credential` command and selected Token backend remain the only product reader;
no independent helper, daemon, or second secret store is admitted.

The macOS worker keeps the published go-keyring logical slot grammar and envelope, but
owns read, write and delete through Security.framework. A private legacy-Keychain
fixture demonstrated that `LAContext.interactionNotAllowed` plus the legacy
per-query UI-fail flag did not prevent an authorization prompt. The
single-operation worker temporarily disables its own optional Keychain UI and
restores that process setting. An item created by the old `/usr/bin/security`
writer may deny direct AIGW access: a successor refuses to overwrite such an
item without authorization. The bridge changes neither ACL nor backend
selection; source tests and dual-architecture cgo builds do not replace
retained-item or signed-artifact acceptance.

The new macOS reader addresses `native@` plus each logical Account slot,
including the separate optional provider-diagnostic slot. Native metadata
observation now requests attributes through Security.framework under the
same no-UI policy; the old `/usr/bin/security` observer is removed.
Native item service, Account and display names have one precise grammar in the
existing secrets owner. Metadata and exact-path cleanup must preserve operator,
predecessor and diagnostic consumers; naming similarity grants no ownership.
Old `/usr/bin/security` items remain at their original addresses for cached
predecessor commands and rollback. This is one backend with separate physical
items across an explicit, one-time authorization transition; neither reader
falls back to the other's item. The candidate must accept a Token supplied
through the existing input path and prove native access before any projection
or package link switches. An enabled diagnostic capability likewise needs
its own credential explicitly staged and qualified before claiming continuity.
Missing input or denied authorization stops the affected cutover without
removing the predecessor item.

Prepare and qualify the successor before changing a client projection. The
one-time 0.3.1 Homebrew-link transition preprojects private paths, prefetches
the package, bounds and measures the unlink/relink window, then verifies old
commands and real clients with immediate rollback on failure. Its cached
public-link risk is disclosed, not called zero interruption. Thereafter keep
every versioned original command callable through replacement. Retain exact
owned bytes for configured, cached, explicit and rollback callers; unknown
consumers block deletion. A fixed path, Unix rename, fresh-client test or green
source gate alone does not establish Windows behavior.

Task 4.5 and final installed acceptance also qualify actual App reload when
needed and retained conversation continuation, including delegated tool use.
CLI success does not qualify a Desktop mode. Preserve native per-conversation
model choice and history; report unsupported modes rather than rewrite metadata.

### 5. One native qualification graph and one publication identity

The repository lockfiles and `mise run bootstrap` reconstruct each Work Lane's
mutable `node_modules` and build state. Shared content-addressed caches are
not mutable cross-lane environments. Task 5.1 tests the environment boundary
in a fresh current-HEAD Git worktree, not ETHOS lease creation. At `1015e56c`,
empty HOME, Mise, Go, and npm caches installed all 19 locked tools and 283 npm
packages; selected Go, Node, npm, and OSV binaries came from the private Mise
root. Authored-input hashes and tracked files were unchanged; the focused
ambient-fallback tests passed. The exact checkout and test state were removed,
including read-only Go module-cache files. This does not prove a formal new
ETHOS Work Lane or any other operating system.

Windows public-fixture preparation belongs to the original native CI job, not a
host transport or drain controller. Exact package/source/checksum inputs select
its private job directory; independently configured trust remains outside the
download. Native glab, tar and locked uv prepare complete client companions and
official Hermes with managed Python 3.12 and its frozen lock. The original release
parser is the sole candidate/lifecycle admission owner. Full-quality and
lock-refresh requests retain source qualification. After actual native consumer
acceptance, remove superseded installer stages; preparation and archive inspection
cannot qualify the Windows service token or client tools.

GitLab source jobs retain the selected acquisition source's locked tool cache;
job-private mirror credentials are removed independently. Isolated fixtures
must not inherit that retained root. Ordinary native source runs retain their
actual verification outputs rather than declare optional child reports as
unconditional artifacts. Prebuilt-only acceptance and failures before an output
is produced may have no verification directory; artifact declarations do not
prove mode-specific evidence or justify empty placeholders.

Existing Go, OpenSpec, CUE, formatting, lint, type/structure, security,
document, link, and supply-chain tools retain
one property owner each; replace hand-written duplicates only after the mature
tool proves the same failure boundary. Complexity and ELOC bounds are blocking
where measured risk warrants them, not lower numbers chosen for appearance.
Warnings fail at their producing owner. Formatting, links, diagrams, examples,
and docs-code correspondence are tested from tracked content.

The existing captured-command runner gives native tools one owned temporary
output file rather than pipe descriptors. A tool that exits immediately may
otherwise lose buffered result or diagnostic bytes. The runner returns both
streams without discarding warnings, preserves the command exit status, and
fails on capture or cleanup errors. The file is removed before return; caller
files and foreign content remain untouched. Native immediate-exit controls and
the unchanged missing-local-OpenSpec test qualify this evidence boundary.

Task 6.4's source gate passed on `b4edd0a9`: tracked Markdown, Mermaid,
local anchors and Git-tracked link targets, structured formats, embedded shell,
and generated projections were checked with their existing negative fixtures.
A separate bounded HTTPS check over 29 current authored documents found 166
successful links, no errors and two redirects; 197 non-HTTPS references were
excluded from that online observation, not counted as online successes. External
availability is time-bound, while the local source gate remains repeatable.

The npm override can advance to jsdom 30.1.1, published 2026-09-22. Its
resolved graph selects `@asamuzakjp/dom-selector` 9.2.2, published
2026-09-26T21:06Z; clean install, `npm audit signatures` (283 signed packages,
51 attestations), vulnerability audit and Mermaid rendering pass. The ordinary
three-day release-age rule held proposal publication until
2026-09-29T21:06Z. Do not add a second override just to evade that bound.
That age rule protects npm's unpublish window; applying it to checksummed Go
modules, locked Mise tools and digest-pinned CI artifacts adds delay without
the same integrity benefit. Those sources instead need exact upstream identity,
compatible behavior and native gate evidence. Select Renovate and Mise images
by upstream release and OCI digest; preserve the Windows Mise ZIP's upstream
SHA-256 through mirroring. Task 6.1 owns complete locked supply-chain
qualification; its current progress belongs only in `tasks.md`.

The CUE graph remains the sole CI intent. GitHub and GitLab are equal optional
peers receiving the same signed Git objects, with separate transport credentials,
native macOS/Linux/Windows jobs, required-status enforcement, and event coverage
for proposal create/update, maintainer integration, dev, main, and tag. Source
proof, review CI, accepted-ref CI, signed/tagged assets, installation, and
real-client operation are distinct evidence. Freeze exact source and lock
inputs before the expensive final matrix; do not rerun identical heavy gates
because an observation timed out or a progress-only record changed.
Review approval or a comment alone does not require another content pipeline.
Reuse requires unchanged verification inputs, including the merge base, and
still-valid required results; matching only the source SHA is insufficient.
Approval remains an independent merge requirement.

Prebuilt native journeys and source qualification have different prerequisites.
The existing release parser owns artifact selection for `ci native` too.
Explicit candidate/tag inputs without full quality or lock refresh select only
the native artifact tool closure and release journey, while ordinary review,
source, and full qualification retain complete bootstrap, tests and scans. No
artifact success replaces failed quality. A newer same-version verifier may
select an exact signed untagged product commit through `--candidate-source`;
default HEAD, matrix signatures, source/provenance checks and clean-source
admission remain unchanged. Mutable refs and unsigned sources fail before
execution. The CUE projection forwards that input without rewriting artifacts.
The existing graph owns canonical check names. Every GitHub dispatch projects
distinct `Manual` identities, so skipped or partial diagnostics cannot replace
required review/push checks. The tag evidence consumer still requires an exact
tag push and the graph's canonical names. GitLab retains complete MR jobs and
MR-pipeline admission; a manual native result is not MR acceptance.

Pre-archive CI evidence qualifies the event-to-check mapping and the exact
candidate review path; it does not claim that a release event has already run.
The guarded maintainer merge to `dev` must preserve the signed object and remove
the proposal source ref on both peers. After OpenSpec archive and proof of the
archived SHA, promote that object to `main` and create its signed release tag;
require the actual peer-local `main` and tag jobs and release assets before
claiming publication. Projection tests and manual runs cannot substitute for
those final event results.

Native CI forwards declarations after `ci native --`; the existing release
construction owner admits candidate and published-predecessor matrices,
verifies their signed source identity, extracts only native bytes and owns
scratch cleanup. Explicit peer and repository inputs select native gh/glab
downloads with prompts disabled and a per-call deadline. Neither CI projection
downloads nor a second platform-specific verifier owns that decision. Real
client paths must be declared before construction or download begins.

Historical GitHub acceptance now invokes the original release command once;
source and artifact trust use `ci trust-input`, and performance uses its native
task. Equal-sign arguments retain intentionally empty optional tags through
PowerShell. Selected client or macOS native-store succession requires a
published predecessor; ordinary isolated lifecycle verification does not.
The scoped official Windows installer retains its pinned six-stage protocol,
locked dependencies and blank Forge-token environment. Its installer file and
client root are created exclusively, registered after creation and removed by
exact owned paths on success or failure. Other platforms do not inherit that
Windows supply block. Native Windows installation, interruption and cleanup
remain distinct from source projection and the local PowerShell argv witness.
The official installer owns its `GIT_CONFIG_COUNT` overrides; caller newline
policy lives in a private Git config inside the same exact supply root instead.
Original Git verifies the pinned HEAD and clean source before client exposure.
That configuration is neither the host's global config nor persistent state.

Linux Secret Service CI uses separate private D-Bus sessions for locked-store
refusal and the unlocked native lifecycle. The locked fixture owns its temporary
HOME, synthetic login collection and controlled native-daemon unlock. A native
method monitor proves that denied operations never request an unlock, display a
prompt or access credential bytes; the fixture also proves pending-prompt
dismissal and retained value and metadata after controlled unlock. The selected
native Go journeys own success and failure; CI forwards their output and exit
directly. Metadata-only existence and already-absent deletion remain available
without unlocking a collection. Unavailable or ambiguous stores fail at the
original credential owner.

Unprotected `proposal/*` reviews must not be protected merely to reach a
protected runner: eligible protected GitLab merge requests receive protected
variables and runners together, while a persistent Shell account retains its
runner credential across jobs. The fork-parent denial and same-project CUE
guard close only the fork path. Complete GitLab review admission requires
disposable, separately identified macOS and Windows MR executors; protected
`dev`/`main`/`v*` jobs may use a distinct protected runner pool only after
event-specific routing is proved. GitHub-hosted checks cannot substitute for
GitLab's required peer-local native evidence. GitLab MR !178 pipeline 8887
ran macOS and Windows review jobs on unprotected project runners #105 and #103
at `ddb998a5`, but success on that old SHA does not prove disposable execution,
absence of persistent Shell credentials, or admission of the final review SHA.
Keep the review path unadmitted until those boundaries are proved.
Windows pre-tool admission verifies the supplied Mise executable and shim;
native `mise install --locked` owns actual shim staging and its failures.
The historical copy/hash/delete experiment is removed without a replacement
controller. Per-job state, mirror authentication and exact cleanup remain
separate from the still-unproved Runner credential-containment boundary.
Cold-cache CI must remain executable when the sibling Forge platform and its
tool-distribution endpoints are unavailable. Warm caches are not evidence of
that property. GitLab consumes Mise's official Debian Docker Hub image pinned
by its multi-platform OCI digest, so image bootstrap does not require GitHub
transport. Locked GitHub Release assets still need an
integrity-preserving independent route. Reuse the selected GitLab project's
Generic Package registry as a lock-digest-addressed mirror. This is the
smallest current route, not a claim that assets may use only HTTP: a separate
HTTPS asset endpoint would improve transport confidentiality but has no
verified deployment, while a public anonymous mirror avoids download secrets
but adds a project, publishing authority and storage retention. Do not put
the tool archives into permanent Git history.

An isolated Mise 2026.9.15 install accepted a checksum-identical OSV Scanner
2.6.0 asset from a loopback mirror without requesting its deliberately corrupted
SLSA file, although both GitHub attestation settings were enabled. URL rewriting
alone therefore does not prove consumer-side provenance verification; a mirror
must retain an independently verified, signed upstream-to-mirror chain or prove
an equivalent native check before it can satisfy the cold-cache requirement.

Mise's [URL replacement](https://mise.jdx.dev/url-replacements.html) must cover
the locked direct-download and API fallback URLs, plus release metadata. A missing
mirror object fails within the selected peer; it must not reintroduce the sibling
Forge through fallback or redirects. The existing CUE mapping owns these rules,
and native consumer tests derive expected paths from the platform lock. A separate
preflight inventory or second mapping authority is unnecessary. Synthetic tests,
registry metadata and actual peer jobs retain their own acceptance scopes.

The selected GitLab origin currently uses HTTP, and the owner accepts that
intranet risk without excluding a future HTTPS asset endpoint. TLS is not a
prerequisite for this deployment; a later HTTPS cutover should change only
transport configuration and trust. [GitLab's package contract](https://docs.gitlab.com/user/packages/generic_packages/)
supports a short-lived same-project CI Job Token for upload and download.
Seed only accepted lock bytes: verify each SHA-256 and every declared upstream
attestation or SLSA proof before upload; reject conflicting bytes at an
existing mirror path. A consumer uses an anchored Mise URL replacement and
job-private `0600` netrc, with no Token in URLs or logs, no cross-host
redirect, and no inherited GitHub authorization. The accepted HTTP risk is
limited to this ephemeral job credential, not a persistent PAT or runner key.
Retire a mirrored lock version only after no active source ref or job consumes
it. CUE configures the mirror before locked installation. Task 7.5 owns the current
inventory and platform evidence; metadata and projection tests alone cannot prove
Job Token downloads, cold-cache peer-outage isolation or provenance checks.

### 6. Delete by consumer and authority

Review source, tests, tools, root and `.config` files, OpenSpec, docs, generated
Forge projections, local worktrees, and publication residue by semantic owner.
Remove a parallel implementation, obsolete prohibition, unused compatibility
reader, or dead wrapper only after its actual consumer is absent and its
replacement passes focused RED/GREEN and native acceptance. Documentation uses
English, precise links, readable diagrams, and a single navigable authority
path. `tasks.md` is the only Change-progress ledger; no secondary status file,
ad hoc framework, or copied task list is introduced.

## Requirement and acceptance routing

The [task groups](tasks.md) below have one primary contract, implementation owner,
and first executable acceptance entry below. These are routes to evidence, not
completion claims: live Provider results, three-platform native runs, exact-SHA
Forge CI, signing, installation, and cleanup remain separate where the task
requires them. `mise run native` means an actual run on every claimed operating
system, not a macOS result reused for Linux or Windows.

| Task(s) | Canonical requirement                                                        | Implementation owner                                   | First acceptance entry                                                      |
| ------- | ---------------------------------------------------------------------------- | ------------------------------------------------------ | --------------------------------------------------------------------------- |
| 1.2     | [organization] · Repository meaning is traceable through one semantic path   | OpenSpec Change                                        | `openspec validate terminal-product-convergence --strict`                   |
| 2.6     | [projection] · Projection conflicts are rejected before configuration writes | `internal/client`, `internal/synchronization`          | `mise exec -- go test ./internal/synchronization ./internal/cli/acceptance` |
| 2.7     | [readiness] · Endpoint observations preserve service independence            | `internal/diagnostics`, `internal/client/verification` | `mise run native`; selected live `aigw verify --for <client>`               |
| 3.1–3.4 | [reader-delta] · Upgrade evidence preserves credential continuity            | `internal/credential`, `internal/synchronization`      | `mise exec -- go test ./internal/credential ./internal/synchronization`     |
| 3.5     | [reader-delta] · Upgrade evidence preserves credential continuity            | `tools/release`                                        | `mise run native` on macOS, Linux, Windows                                  |
| 3.6     | [client-delta] · Independently admitted native clients                       | `internal/client`, `internal/credential`               | `mise run native`                                                           |
| 4.1     | [control] · Model admission follows protocol and capability                  | `internal/configuration`, `internal/providers`         | `mise run check`                                                            |
| 4.2     | [control] · Provider catalogue evolution is observed before admission        | `manifests/team.toml`, `internal/providers`            | `mise run native`; selected live Route probe                                |
| 4.3     | [selection] · Provider preference is advisory control-plane input            | `internal/configuration`, `internal/activation`        | `mise exec -- go test ./internal/configuration ./internal/activation`       |
| 4.4     | [control] · Real Codex client route verification                             | `internal/codex`, `internal/client/verification`       | `mise run native`; `aigw verify --for codex`                                |
| 4.5     | [client-delta] · Independently admitted native clients                       | `internal/client/verification`                         | `mise run native`; real-client `aigw verify`                                |
| 4.6     | [client-delta] · Independently admitted native clients                       | `internal/client` Adapter registry                     | Source-backed admission; `mise run native` if admitted                      |
| 4.7     | [control] · Independent product composition                                  | `internal/configuration`, `internal/client`            | `mise run native` without Proxy or Forge                                    |
| 5.1     | [organization] · Each Work Lane reconstructs its own mutable environment     | `mise.toml`, committed locks                           | `mise install --locked`; `mise run bootstrap`                               |
| 5.2–5.4 | [control] · Native released-artifact lifecycle acceptance                    | `tools/release`, `internal/secrets/native`             | `mise run native` on macOS, Linux, Windows                                  |
| 5.5     | [onboarding] · Activation follows present capabilities                       | `internal/discovery`, `internal/synchronization`       | `mise run native`                                                           |
| 5.6     | [control] · Failure recovery retains owned-resource cleanup outcomes         | `internal/transaction`, `tools/release`                | `mise run native` with failure injection                                    |
| 6.1     | [control] · Latest stable repository-owned supply chain                      | `mise.toml`, locks, dependency policy                  | `mise run dependencies:check`; `mise run check`                             |
| 6.2–6.3 | [quality] · Quality constraints are comprehensive and proportionate          | `.config/checks`, `tools/ci`                           | `mise run check`                                                            |
| 6.4     | [quality] · Repository text quality has one mature owner per concern         | `.config/checks`, `tools/ci/markdown`                  | `mise run check`; online link proof remains separate                        |
| 6.5     | [quality] · Dependency evidence binds the selected lockfiles                 | `tools/ci`, `tools/release`                            | `mise run check`; build-only `mise run release`                             |
| 6.6     | [quality] · Quantitative policy is evidence-derived                          | `tools/release` performance owner                      | `mise run performance` with a signed candidate                              |
| 6.7     | [quality] · Warnings are owned failures                                      | `internal/presentation`, `tools/ci`                    | `mise run check`                                                            |
| 7.1–7.3 | [ci] · One CI graph projects to independent Forges                           | `.config/ci/pipeline.cue`, `tools/ci`                  | `mise run check`; peer-native jobs separately                               |
| 7.4–7.5 | [ci] · Every integration path produces exact-commit evidence                 | `tools/forge`, peer policies                           | `mise run native`; exact-SHA peer/API proof separately                      |
| 8.1     | [organization] · Logical and physical ownership are isomorphic               | Go packages, `.config/checks/architecture`             | `mise run check`                                                            |
| 8.2–8.3 | [organization] · Semantic documentation architecture                         | `docs/`, `tools/ci/markdown`                           | `mise run check`                                                            |
| 8.4     | [quality] · Engineering-reference quality is demonstrated by behavior        | `docs/decisions/`, affected package owner              | Source-backed comparison; `mise run check` if adopted                       |
| 8.5     | [quality] · Delivery completion is evidence-bound                            | Git common-dir, release/ETHOS owner                    | `git worktree list --porcelain`; exact residue audit                        |
| 8.6     | [organization] · Source-owned architecture inputs have one authority         | `architecture/edition-provider`                        | Native source tests; exact published Provider v2 offline replay             |
| 9.1–9.2 | [quality] · Source acceptance precedes delivery completion                   | OpenSpec, `tools/ci`                                   | `mise run check`; strict OpenSpec validation                                |
| 9.3     | [control] · Native released-artifact lifecycle acceptance                    | `tools/release`                                        | Build-only `mise run release`; `mise run native` on all three OSs           |

[organization]: ../../specs/repository-organization/spec.md
[projection]: ../../specs/projection-format/spec.md
[readiness]: ../../specs/cli-readiness/spec.md
[reader-delta]: specs/secret-storage/spec.md
[client-delta]: specs/product-control-plane/spec.md
[control]: ../../specs/product-control-plane/spec.md
[selection]: specs/route-client-selection/spec.md
[quality]: ../../specs/product-quality/spec.md
[ci]: ../../specs/ci-diagnostics/spec.md
[onboarding]: ../../specs/progressive-team-onboarding/spec.md

### Repository identity and release history

The one [Changelog](../../../CHANGELOG.md) keeps neutral local version headings
and offers each applicable peer's native history.
[Release declarations](../../../.ethos/release.toml) own `forge_repository`
independently of SSH or HTTP Git transport; no inferred scheme, port, user path,
or second local identity manifest is needed. The current untagged prepared
release compares the latest published tag with `main` and labels those links
as prepared changes. Final release navigation is frozen in the signed release
source and verified on each peer after publication; never edit source behind
an existing tag. A history link cannot manufacture an old Release merely
because a local Git tag exists.

The native release parser retains strict SemVer, dates, categories, and exact
source/tag responsibilities. ETHOS owns common repository identity, reference
membership, and authorized historical repair; no generic checker is copied here.
Current source navigation, installed product prevention, cross-peer tag
reconciliation, and publication remain independently qualified.

## Risks / Trade-offs

- Original credential commands may outlive the configuration that created
  them. Premature byte removal breaks sessions; indefinite retention is not a
  terminal state. Exact ownership and a proved transition govern cleanup.
- Client and Provider capabilities differ. A common adapter contract reduces
  core edits but cannot make unsupported native protocols work by declaration.
- Provider catalogues drift faster than releases. Live probes qualify selected
  claims; a model name alone cannot establish inference, tool use, or a 1M
  context window.
- A broad Change can hide partial completion. Each task therefore closes one
  independently testable semantic behavior and names its native evidence;
  task count is not a readiness percentage.
- Locally accepted Hermes source `2aa58eea` has exact-HEAD ETHOS proof
  Attestation `731c06ac` and a [GitHub proposal Verify run](https://github.com/HengYangDS/aigw-cli/actions/runs/36234147864)
  at the same SHA. Its OpenSpec record is archived in this terminal Change;
  equivalent later gate wiring belongs here, not in a second Hermes landing.
  Reconcile foreign work at its owning requirement and rerun only evidence
  invalidated by changed inputs.

## Migration Plan

Resolve accepted source and lane authority from current ETHOS state. This Change
closes the user journey and reader
succession before changing live client projections, and applies the broader
adapter, quality, topology, and documentation cleanup in dependency order.
Use a published predecessor with retained state for each platform's update and
rollback proof. Pre-archive candidate acceptance uses explicit environment
credentials or isolated synthetic native items. A Developer ID candidate may
prove stable designated requirements without a tag, but signing does not move
the predecessor's physical item or prove access to an operator's Token.
After the final candidate checkbox is committed, obtain exact-HEAD ETHOS proof
and independent peer review CI, resolve their gaps, then integrate the same
signed object into `dev` through the guarded maintainer path on both peers.
Verify the resulting dev checks and proposal-source-ref deletion before
archiving through the official governed OpenSpec transition. Re-prove the
archived SHA; only then promote that exact object to `main`, require its peer-local
main checks, and create the signed release tag. The tag jobs and dual-peer
assets must pass before Homebrew update or user-host cutover can be claimed.
Housekeeping completes at each operation boundary, not in a final catch-all
sweep. Before archive, Task 8.5 inventories exact ownership, removes currently
disposable residue and identifies retained consumers and retirement triggers.
After verified release and installed cutover, retire superseded tags, outputs
and the completed lane through their native owners. Preserve failed receipts,
required rollback material and foreign or unknown state. These final retirements
remain delivery requirements but cannot be prerequisites for their own release.
Proof and archive cannot be checkboxes in the Change they finalize, because
checking either box changes the HEAD it would claim to have proved.
Before that host cutover, the exact final production reader must authorize each
selected Account and diagnostic item, preserve captured original commands and
rollback, and qualify the measured package-link transition. Ad-hoc candidate
success, an unchanged signer or an item label cannot substitute for that read.
The one-time 0.3.1 Homebrew transition must preproject versioned commands,
prefetch the final package, measure the bounded unlink/relink gap, verify the
captured original commands immediately, and restore the predecessor on failure.
Retain old items and readers for cached and rollback callers; disclose residual
cached-public-link risk. Later versioned commands remain callable throughout
replacement. These are post-archive acceptance conditions, not prerequisites
for the pre-archive candidate tasks.
No credential prompt, service restart, client history rewrite, or unverified
automatic backend fallback is a migration step.
