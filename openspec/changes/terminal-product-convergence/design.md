# Design

## Context

See [the proposal](proposal.md). The accepted base is signed `2aa58eea`; its
Hermes delivery Change is being archived in a separate owned Work Lane. This
Change may plan and test independently, but must refresh onto the later accepted
archive before integration. The foreign architecture-edition Work Lane remains
untouched until its holder provides a native handoff or its content is accepted
through the normal lifecycle.

The installed 0.3.1 program remains the protected predecessor. Current source
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

The existing manifest remains token-free team policy. Each Account Token is
optional until a selected operation requires it. Native keyring, guarded file,
and process environment are alternative backends with one recorded owner per
invocation; an unavailable backend fails explicitly and never silently falls
through to another. Client discovery never creates a missing client's files.

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

### 4. Succeed the credential reader without a launcher

The fixed private copy currently avoids a Homebrew-link gap but never upgrades
after its first creation. Replace that implicit forever-version with immutable
reader generations keyed by verified program identity. The existing
`credential` and `synchronization` transaction owners prepare the successor,
prove executable integrity and native Token access, then compare-and-swap only
the selected AIGW-owned client projections. An old loaded command keeps its
original bytes and Token behavior. Failed projection or reader qualification
preserves the old command and leaves no unconsumed new reader. Rollback selects
the previous qualified generation; uninstall removes only exact unconsumed
owned generations.

There is no cross-platform promise of atomically replacing a running
executable: Windows may keep it open, while a Unix symlink or shell launcher
would introduce a second indirection and would not solve cached client
endpoints. An older generation remains active state until its configured,
cached, explicit, and rollback consumers are absent or safely transitioned.
Unknown consumers block deletion; age or filename prefix never proves absence.
The current CLI path remains package-manager-owned, not a credential reader.

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

### 6. Delete by consumer and authority

Review source, tests, tools, root and `.config` files, OpenSpec, docs, generated
Forge projections, local worktrees, and publication residue by semantic owner.
Remove a parallel implementation, obsolete prohibition, unused compatibility
reader, or dead wrapper only after its actual consumer is absent and its
replacement passes focused RED/GREEN and native acceptance. Documentation uses
English, precise links, readable diagrams, and a single navigable authority
path. `tasks.md` is the only Change-progress ledger; no secondary status file,
ad hoc framework, or copied task list is introduced.

## Risks / Trade-offs

- Versioned readers retain bytes during a real cached-client transition;
  premature deletion breaks sessions, while indefinite retention is not an
  acceptable terminal state. Exact ownership and a proved transition govern GC.
- Client and Provider capabilities differ. A common adapter contract reduces
  core edits but cannot make unsupported native protocols work by declaration.
- Provider catalogues drift faster than releases. Live probes qualify selected
  claims; a model name alone cannot establish inference, tool use, or a 1M
  context window.
- A broad Change can hide partial completion. Each task therefore closes one
  independently testable semantic behavior and names its native evidence;
  task count is not a readiness percentage.
- The current Hermes archive and the foreign architecture lane may alter the
  base. Refresh from accepted truth, reconcile overlap at the owning requirement,
  and rerun only evidence invalidated by changed inputs. Do not reset, cherry-pick
  unseen work, or publish a second proposal for the same semantic content.

## Migration Plan

First land the already-reviewed Hermes archive on its own lane. This Change
then refreshes to that accepted source, closes the user journey and reader
succession before changing live client projections, and applies the broader
adapter, quality, topology, and documentation cleanup in dependency order.
Use a published predecessor with retained state for each platform's update and
rollback proof. Only after final exact-HEAD proof and OpenSpec archive may a
signed tag, dual-peer assets, Homebrew update, and user-host cutover be claimed.
No credential prompt, service restart, client history rewrite, or unverified
automatic backend fallback is a migration step.
