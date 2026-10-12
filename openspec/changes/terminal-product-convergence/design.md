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

| Order | Semantic closure                                                                | Existing task owners                            | Exit condition                                                                                                                                                          |
| ----- | ------------------------------------------------------------------------------- | ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Close client-scoped forwarding and its original configuration/projection owners | 4.7, 6.3, 9.1, 9.2                              | Retained reader, generation-bound compensation, stale-selection refusal, credential-free preview, human/JSON continuation and native size limits pass on stable inputs. |
| 2     | Prove effective peer admission, runner containment and cold supply custody      | 7.4                                             | Required checks, exact source-ref retirement policy and per-job isolation are enforced independently on each selected peer.                                             |
| 3     | Consume the accepted ETHOS identity/reference successor                         | 8.3.2                                           | The installed adopter rejects wrong-repository references and qualifies authorized history repair without a private checker.                                            |
| 4     | Freeze one signed candidate for native-client and affected-path acceptance      | 4.5, 4.7, 9.3                                   | Exact macOS/Linux/Windows bytes pass required retained-predecessor, store, client-mode, forwarding, rollback, forward and cleanup journeys.                             |
| 5     | Deliver and retire AIGW through the Migration Plan                              | Migration Plan                                  | Signed integration, archive, publication, package-manager cutover and installed acceptance pass; owned obsolete consumers retire.                                       |
| 6     | Resume the existing provider-neutral Responses Proxy closure                    | Existing Proxy Change and its authorized holder | Replay, transport, native lifecycle, repository quality, release and installed-product evidence pass without AIGW coupling.                                             |

Only the accepted-ETHOS-dependent scope waits for its successor. Advance
independent candidate preparation without competing host assays. A prose-only
commit needs fresh exact-HEAD governance, not rebuilt unchanged product bytes.
Tasks 9.1 and 9.2 qualify frozen inputs; product or verifier changes reopen that
qualification. Reuse unchanged 3.5, 5.2 and 6.6 evidence only within its bound
scope; new forwarding paths and final signed bytes require their own acceptance.

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

Team curation follows the [reviewed defaults](../../../docs/guides/team-rollout.md#reviewed-model-defaults)
and [dated qualification](../../../docs/research/provider-model-qualification.md#october-10-haiku-55-admission).
The October 10 GPT 6.1+/Claude 5.5+ floors supersede older catalogue inclusions;
requested successors without qualified wire IDs and inference remain unadmitted,
not permanently unsupported. These authored team constraints do not alter native
conversation model choices or become Provider-specific runtime admission rules.

Account and Model ranks are optional presentation metadata, not a second
recommendation or failover policy. The Store owns `config.catalog.toml` alongside
its guarded backups; canonical routing and credential readers never depend on
it. Verified checkpoints restore historical routing with current presentation
for surviving identities. Ordinary synchronization rejects managed Hermes
drift; explicit Hermes selection may adopt only a proved native selection
change, then must update ownership even if the native file already matches.
Rollback restores both observed preimages. Native picker policy remains the
client's authority; configured order alone is not rendered-order evidence.

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

Private Hermes verification homes stay outside both native data roots, borrowing
`facts.json`, environments, PM runtime, physical `.install.lock` and tools.
Missing locks refuse launch; Hermes owns leases. No upstream isolation API.

`PrepareVerification` disables lazy installs, update checks and PATH exposure.
Hermes 0.21.6 ignores the last on Windows; AIGW refuses before launch.

The [boot regression](../../../internal/client/hermes_native_test.go) exercises
Hermes' native record resolver with boot steps stubbed. The
[custody regressions](../../../internal/hermes/configuration/verification_test.go)
check physical-lock identity, refusal and containment; a real journey separately
qualifies launch effects.

Signed `2b746c9610424f0071bdcf4e5a2900fc08f5af3d` produced a six-target candidate.
Installed macOS Codex, Claude Code and Hermes independently passed packaged
0.3.3 upgrade/rollback/re-upgrade, forwarding and retained credentials with public
Tokens/local servers. Codex tools and Hermes continuation/model adoption passed;
ten general Routes remained metadata-unqualified. All 85,992 Hermes dependency/tool
nodes and operator state stayed intact except a lease-directory mtime; no residues.

Current signed `8254c1f5` produced a refreshed six-target matrix. Its source tree
`66591638` matches all 1,248 accepted inputs: Go 1.27.2 measures 95.01% statement
coverage, above the unchanged strict 95% floor. Artifact checksum signing is
distinct from Developer ID signing and notarization; this candidate uses ad-hoc
macOS signatures.

The current native boot regression and packaged macOS Hermes journey passed,
including `verify --for hermes`, streaming tools, continuation, native model
adoption, forwarding and the four retained-predecessor lifecycle stages. All eight
operator files and the default boot record stayed byte/mode/inode intact; all
86,077 dependency/tool nodes remained, with only native lease-directory mtime
changed. The authorized 36 residue records remain absent. Hermes' existing
dependency-lock warning persists; this is not warning-free installation or
full product acceptance. Windows verification still refuses before launch.

Evidence: `build/verification/<source>/native-{candidate,hermes-boot,hermes-artifact}-20261012`
for `8254c1f5`; the original `2b746c96` Codex, Claude and Linux scopes retain their
own receipts. Full source evidence remains under
`build/verification/5c64433d1a9f081d0c0ec3b530c56f6b906cbc54/supplier-refresh-preflight-20261012`; final-byte,
native-store, Desktop, Windows and clean-supply obligations remain open.

An Account endpoint declares upstream service identity. An explicitly selected
Client Binding may declare a forwarding endpoint without changing that Account,
Route, provider identity or another client. The existing resolver derives the
effective client destination, and the native CLI, credential reader and guarded
projection consume that same binding. Preserve the cached predecessor's direct
credential reference without copying its Token or weakening endpoint admission.
No second controller, provider alias or manually patched sidecar owns this
choice. Acceptance requires credential-free preview, fresh and retained-child
tool loops through installed forwarding bytes, converged sync, rollback and
exact cleanup; source mapping or a reachable listener alone is insufficient.

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

The macOS reader, writer, deleter and metadata observer preserve the
predecessor's exact Account slot and its separate optional `diagnostic@` slot.
Metadata observation requests attributes through Security.framework under the
same no-UI policy; the old `/usr/bin/security` observer is removed. Updating the
reader must not require Token enrollment again or create a second physical item.
A predecessor rotation after rollback is visible to the successor before any
projection or package-link switch. Denied authorization blocks only the affected
transition; no ACL mutation, backend fallback or Token copy is permitted.
Native item service, Account and display names have one precise grammar in the
existing secrets owner. Preserve exact shadow items made by unreleased candidates
until their retained consumers and rollback owners release them; naming similarity
never grants cleanup authority.

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

#### Environment and supply ownership

Lockfiles and `mise run bootstrap` reconstruct each checkout's mutable environment;
shared content-addressed caches are not shared mutable Work Lane state. The
`1015e56c` empty-HOME/cache bootstrap qualified one fresh Git worktree, its
unchanged authored inputs and exact cleanup, not ETHOS lease creation or another OS.
Native npm commands select the tracked registry, reject ambient operator settings,
and use checkout/job-owned caches. Isolation under one OS identity is not containment.

Use the latest compatible stable pins. The npm three-day publication delay protects
its unpublish window; Go modules, Mise tools and OCI images instead require exact
upstream identity, lock/checksum or digest, compatible behavior and native evidence.
The resolved jsdom 30.1.1 graph passed clean installation, signatures and Mermaid
rendering; its 9.2.2 selector's publication delay ended 2026-09-29T21:06Z.
The npm 12.2.0 report retains twelve advisory occurrences in five bundled components;
reviewed command use is not universal tool safety. Findings are nonblocking;
malformed evidence, scanner failures, integrity and authenticity remain required.

#### Quality and evidence ownership

Existing Go, OpenSpec, CUE, format, structure, security, link and rendering tools
own distinct properties; replace a duplicate only after equivalent failure evidence.
Retain risk-justified complexity, size and performance bounds and fail native warnings.
The captured-command runner retains both streams and exit status in an owned file,
then removes it before return; capture or cleanup failure remains a failed command.
Source jobs retain acquisition tool caches, remove job-private mirror credentials,
and publish only reports actually produced. Optional artifacts need no empty placeholder.

Source and online documentation checks are distinct: `b4edd0a9` qualified tracked
Markdown, Mermaid, anchors, tracked targets and structured/embedded formats; its
bounded HTTPS check covered 166 successful links, two redirects and no errors in
29 documents, excluding 197 non-HTTPS references. Availability remains time-bound.

#### Peer admission and native candidates

The [CUE graph](../../../.config/ci/pipeline.cue) is the sole CI intent for equal optional
GitHub/GitLab peers. Each owns exact signed objects, transport authentication,
three-OS jobs, required checks and proposal/update/maintainer/dev/main/tag events.
Approvals alone do not trigger duplicate content CI; reuse binds unchanged inputs,
merge base and still-valid results, not SHA equality alone. Manual GitHub check names
cannot replace canonical review/push/tag checks; GitLab MR admission remains peer-local.

The release parser owns `ci native` candidate/predecessor matrices, signatures,
provenance, native extraction and scratch cleanup. Prebuilt acceptance selects its
artifact tool closure; ordinary source/full-quality/lock-refresh retains full bootstrap
and scans. `--candidate-source` may select an exact signed untagged product with a
same-version verifier; mutable refs, unsigned sources and failed quality remain refused.
Declare real-client inputs before construction/download; gh/glab are deadline-bounded
and no-prompt. [Delivery](#migration-plan), not a policy readback, proves integration,
proposal retirement, archive, main/tag events, assets and installed cutover.

Unprotected proposals must not gain protected credentials by using protected runners.
The [native review boundary](../../../docs/governance/change-and-release-policy.md#commit-and-tag-identity)
requires OS-enforced separation from Runner-manager/protected-job state, actual denial
and per-job cleanup; cleared environments, separate IDs or another peer's pass are
insufficient. Protected dev/main/tag pools require event-specific routing evidence.

Windows supply stays in the original native CI owner: exact source/packages/checksums,
verified Mise executable/shim and native locked installation; explicit client selection
alone prepares companions and official Hermes/Python 3.12/uv dependencies. Its official
six-stage installer uses exclusive owned paths, blank Forge-token state, private Git
newline policy and exact pinned clean HEAD. Native token/client/interruption/cleanup
acceptance is distinct from source projection; no host transport/drain controller.
Linux native-store refusal and unlocked lifecycle use separate private D-Bus sessions,
HOME and synthetic collections. Native monitoring proves no unlock/prompt/value read
on denial, pending-prompt dismissal and preserved data after controlled unlock.
Metadata-only existence/absent deletion remain available without unlocking.

#### Independent cold supply

A peer outage must not disable the sibling's cold install. GitLab's digest-pinned
Debian Mise image avoids GitHub bootstrap; its Generic Package mirror uses the exact
SHA-256 of `.config/mise/mise.lock` as version, CUE-owned URL mappings and private Mise caches.
Verify archive hashes and every declared upstream attestation/SLSA proof before upload;
refuse conflicting existing bytes. The accepted intranet HTTP risk covers only an
ephemeral same-project CI Job Token via private `0600` netrc, never persistent keys,
URLs/logs or inherited GitHub authorization. HTTPS may change transport, not authority.

Cover direct-download/API/metadata fallback paths; missing objects must fail locally.
Mise URL replacement alone proves neither provenance nor redirect containment:
2026.9.15 accepted a loopback OSV 2.6.0 asset without its corrupted SLSA file;
2026.10.2 allows cross-host same-scheme redirects with no native allowlist. Qualification
needs anchored provenance and direct `200` cold downloads with sibling endpoints
unavailable, or an existing executor network boundary. Task 7.5 owns native isolation;
synthetic tests and registry metadata cannot substitute. Retire mirror versions only
when no active source ref or job consumes them; never store archives in Git history.

#### Reuse only qualified scopes

Retain immutable native lifecycle and performance receipts under their exact
source-bound verification owner. Reuse requires matching product inputs,
artifact digest, verifier/controller, commands, OS, backend and retained state;
another tree's checkbox cannot supply that binding. Changed forwarding or
catalogue inputs require affected acceptance, not a wholesale historical rerun.
Controlled native measurements qualify their declared budgets and cleanup,
not Provider latency, another platform, a quiet host or optimization benefit.
Build and CI costs remain with their actual construction and job receipts;
unmatched timing is neither optimization nor regression evidence.

The `2b746c96` candidate passed macOS ENV and Linux ARM64 ENV/file/Secret Service
budgets in `macos-env-performance-20261012` and `linux-native-store-performance-20261012`
under that source's verification root. Raw samples, controllers and memory remain.
These bound overhead, not speedup: macOS setup/sync/projection/status are slower
in both blocks, with observed contention. Linux locked-store/refusal/lifecycle passed;
supplier registration warnings remain disclosed, not a pristine aggregate gate.
`native-lifecycle-20261012` has one Desktop skip; Linux has none. Windows, macOS
Keychain, final Developer ID bytes and package-link cutover remain open.

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
| 5.1     | [organization] · Each Work Lane reconstructs its own mutable environment     | `.config/mise/config.toml`, committed locks            | `mise install --locked`; `mise run bootstrap`                               |
| 5.2–5.4 | [control] · Native released-artifact lifecycle acceptance                    | `tools/release`, `internal/secrets/native`             | `mise run native` on macOS, Linux, Windows                                  |
| 5.5     | [onboarding] · Activation follows present capabilities                       | `internal/discovery`, `internal/synchronization`       | `mise run native`                                                           |
| 5.6     | [control] · Failure recovery retains owned-resource cleanup outcomes         | `internal/transaction`, `tools/release`                | `mise run native` with failure injection                                    |
| 6.1     | [control] · Latest stable repository-owned supply chain                      | `.config/mise/config.toml`, locks, dependency policy   | `mise run dependencies:check`; `mise run check`                             |
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
Source acceptance is distinct from final product delivery. Obtain exact-HEAD
ETHOS proof and independent peer review CI for each source increment, then
integrate its signed object through the guarded maintainer path. Keep future
native acceptance and delivery tasks open; they cannot block the source or
transport policy needed to execute them. Verify dev checks and exact proposal
ref retirement for each effect. Once candidate acceptance is complete, archive
through the official governed OpenSpec transition and re-prove the archived
SHA. Only then promote that exact object to `main`, require its peer-local main
checks, and create the signed release tag. Tag jobs and dual-peer assets must
pass before Homebrew update or user-host cutover can be claimed.
Housekeeping completes at each operation boundary, not in a final catch-all
sweep. Before archive, Task 8.5 inventories exact ownership, removes currently
disposable residue and identifies retained consumers and retirement triggers.
Native input acquisition belongs to the existing release owner. GitLab jobs
consume the exact source-bound package; local jobs may consume its absolute
caller-owned archive. Both paths retain checksum, signed matrix, provenance and
cleanup checks without a transport tag. Neither qualifies GitHub's disposable
macOS Keychain consumer. That consumer still needs a same-peer input transport
admitted by its prior accepted policy; draft Release visibility is not a
supported contract for its read-only job token. An accepted signed transport
carrier may descend from the package producer; its tag names and selects that
carrier, while `CandidateSource` and matrix provenance continue to identify the
original producer. Verify their ancestry and exact checksum without treating
the carrier as a rebuilt product. Preserve the measured product-input
equivalence; changed product inputs require new producer provenance. Do not
rebuild unchanged signed inputs or create redundant transport entities. Retire
exact temporary peer inputs only after their consumers terminate and required
acceptance evidence is retained.
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
Brief interruption is acceptable only when the native consumer recovers
automatically, the same thread and turn finish, context and all outputs survive,
and each tool effect executes once. Measure the cutover window and verify that
continuation immediately; a restarted task, lost output or duplicate effect
fails acceptance and requires bounded rollback. Controlled outage evidence
does not replace the final-byte package-manager transition.

Retain old items and readers for cached and rollback callers; disclose residual
cached-public-link risk. Later versioned commands remain callable throughout
replacement. These are post-archive acceptance conditions, not prerequisites
for the pre-archive candidate tasks.
No credential prompt, service restart, client history rewrite, or unverified
automatic backend fallback is a migration step.
