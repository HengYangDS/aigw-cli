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

**Goals:** one coherent setup-to-use journey; explicit provider/client
extensibility; safe credential-reader succession; qualified native behavior on
macOS, Linux, and Windows; one effective quality and CI graph; and smaller,
navigable source, test, configuration, and documentation surfaces.

**Non-goals:** request transport, transparent mid-request retries, external
Proxy lifecycle, Codex conversation or model-choice mutation, automatic browser
or Keychain authorization, a universal client config patcher, a new daemon,
or a second OpenSpec/ETHOS command plane.

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

Prepare and qualify the successor before changing a client projection. The
one-time 0.3.1 Homebrew-link transition preprojects private paths, prefetches
the package, bounds and measures the unlink/relink window, then verifies old
commands and real clients with immediate rollback on failure. Its cached
public-link risk is disclosed, not called zero interruption. Thereafter keep
every versioned original command callable through replacement. Retain exact
owned bytes for configured, cached, explicit and rollback callers; unknown
consumers block deletion. A fixed path, Unix rename, fresh-client test or green
source gate alone does not establish Windows behavior.

### 5. One native qualification graph and one publication identity

The repository lockfiles and `mise run bootstrap` reconstruct each Work Lane's
mutable `node_modules` and build state. Shared content-addressed caches are
not mutable cross-lane environments. Existing Go, OpenSpec, CUE, formatting,
lint, type/structure, security, document, link, and supply-chain tools retain
one property owner each; replace hand-written duplicates only after the mature
tool proves the same failure boundary. Complexity and ELOC bounds are blocking
where measured risk warrants them, not lower numbers chosen for appearance.
Warnings fail at their producing owner. Formatting, links, diagrams, examples,
and docs-code correspondence are tested from tracked content.

The CUE graph remains the sole CI intent. GitHub and GitLab are equal optional
peers receiving the same signed Git objects, with separate transport credentials,
native macOS/Linux/Windows jobs, required-status enforcement, and event coverage
for proposal create/update, maintainer integration, dev, main, and tag. Source
proof, review CI, accepted-ref CI, signed/tagged assets, installation, and
real-client operation are distinct evidence. Freeze exact source and lock
inputs before the expensive final matrix; do not rerun identical heavy gates
because an observation timed out or a progress-only record changed.
Cold-cache CI must remain executable when the sibling Forge platform and its
tool-distribution endpoints are unavailable. Warm caches are not evidence of
that property. Compare neutral locked sources, peer-local immutable assets, and
runner seeds before choosing the least complex integrity-preserving route.

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
| 9.1–9.2 | [quality] · Source acceptance precedes delivery completion                   | OpenSpec, `tools/ci`                                   | `mise run check`; strict OpenSpec validation                                |
| 9.3     | [control] · Native released-artifact lifecycle acceptance                    | `tools/release`                                        | Build-only `mise run release`; `mise run native` on all three OSs           |
| 9.4     | [quality] · Delivery completion is evidence-bound                            | ETHOS proof, peer CI                                   | `ethos prove --execute --expect-head <SHA>`; peer-native CI separately      |
| 9.5     | [distribution] · Stable publication follows completed change acceptance      | OpenSpec, `tools/release`                              | Strict OpenSpec validation before official archive                          |

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
[distribution]: ../../specs/release-distribution/spec.md

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
rollback proof. Only after final exact-HEAD proof and OpenSpec archive may a
signed tag, dual-peer assets, Homebrew update, and user-host cutover be claimed.
No credential prompt, service restart, client history rewrite, or unverified
automatic backend fallback is a migration step.
