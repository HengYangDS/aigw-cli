## Context

See [proposal.md](proposal.md) for motivation. AIGW currently has a working
signed release and substantial contracts for Accounts, Profiles, per-client
Routes, credential backends, Codex and Claude projections, native packaging,
and dual-Forge delivery. The remaining risk is not absence of machinery but
overlapping semantics, uneven proof, repository topology that reflects past
implementation steps, and generic checks maintained beside mature tools.

The current installed product is the rollback boundary. Source migration must
not mutate user configuration, credentials, installed clients, or an external
gateway until a released candidate has passed the corresponding acceptance
journey.

## Goals / Non-Goals

**Goals:**

- Make the path from team manifest to one usable client predictable even when
  other Providers, clients, native stores, or Forges are absent.
- Give each behavior, policy, state transition, file family, and proof one
  semantic owner.
- Reduce source, configuration, and operational entities while increasing
  coverage and native evidence.
- Make a normal Provider extension data-only and a client extension one narrow
  Adapter plus a common conformance suite.
- Deliver one signed object through local, GitHub, and GitLab surfaces without
  confusing transport identity with product identity.

**Non-Goals:**

- AIGW will not carry model traffic, supervise another product, own client
  history, or reproduce ETHOS repository lifecycle.
- This Change will not adopt a second task runner, package manager, workflow
  model, credential database, or evidence store.
- Historical intermediate artifacts will not be retained merely because they
  once existed.

## Authority inventory

The inventory is rule-based rather than a second per-file ledger. Every tracked
path matches exactly one responsibility class in
`.config/checks/architecture/policy.toml`; overlapping matches are errors, not
precedence rules. Go code narrows further through the declared package topology
and import direction. An unlisted peer carrier is a gate failure. A generated
projection is never authoritative and points back to the source named below.

| Surface                                                   | Semantic owner and source of truth                                                            | Consumers and dependency direction                                               | Change and retirement rule                                                                                 |
| --------------------------------------------------------- | --------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Product behavior                                          | The narrow `internal` domain package; `cmd/aigw` only composes it                             | CLI journey → domain → client or Provider leaf                                   | Change for a product invariant; retire when no invariant, caller, or test consumes it                      |
| Public commands, options, and results                     | The matching `internal/cli/<journey>` constructor and result type                             | Human and JSON clients → CLI owner → domain owner                                | Change with the journey contract; retire together when the journey is removed                              |
| Configuration, manifest fields, and environment variables | `internal/configuration`, the reviewed manifest, or the domain package declaring the variable | Input → validation → typed model → transactional projection                      | Change only with the owning schema or capability; retire after supported state has no reader               |
| Client and native resources                               | The admitted client Adapter plus `internal/platform` for OS-neutral facts                     | Configuration → complete plan → atomic OS/client projection                      | Change for an admitted client contract; remove exactly owned state on disable or uninstall                 |
| Repository tools and quality policy                       | The semantic `tools/<concern>` package plus `.config/checks/<concern>`                        | Policy → focused tool → `mise` gate → CI                                         | Change for a measured repository risk; retire when a mature owner supersedes it or the risk disappears     |
| CI projections                                            | `.config/ci/pipeline.cue`                                                                     | CUE model → `.github/workflows/*` and `.gitlab-ci.yml` → Forge runners           | Change in CUE; regenerate projections; retire a job when it proves no unique fact                          |
| Release identity and artifacts                            | `VERSION`, `CHANGELOG.md`, `.config/release`, and `tools/release`                             | Version and source object → deterministic artifacts → Forge Releases → installer | Change once per release identity; retire unreferenced failed artifacts and obsolete tags                   |
| Product intent, documentation, and journeys               | Active OpenSpec requirements plus the nearest canonical documentation entry point             | Product semantics → acceptance tests and user/contributor guidance               | Reconcile with implementation in the same Change; archive intent and delete stale guidance when superseded |
| Team configuration                                        | `manifests/team.toml` without credentials                                                     | Reviewed capability → setup import → local Account, Profile, and Route state     | Change when team capability changes; retire examples or fields without an active consumer                  |
| Repository governance and local exclusions                | `.ethos`, `AGENTS.md`, Git metadata files, and `.gitignore`                                   | Repository policy → developer and agent entry points                             | Change only at the owning governance boundary; remove obsolete exceptions and host-tool residue rules      |

This resolution also covers generated host projections: Codex and Claude files
are transactional outputs of their Adapters, while build products and Forge
files are outputs of the release and CI sources above. Host caches, Tokens,
installed binaries, and Forge observations are evidence or state, never a
tracked source of truth.

### Reconciliation findings

The baseline found the following concrete disagreements. Each is assigned to
one existing closure rather than spawning another plan or compatibility path.

| Current disagreement                                                                                            | Owning closure                                |
| --------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| Direct pushes to `dev` do not currently enter either Forge verification graph                                   | 10.3 event coverage                           |
| Architecture scanning still exempts historical `records` and nested `runtime` names                             | 7.6 and 13.1 residue removal                  |
| Host-local semantic indexes other than `.serena` are not explicitly excluded                                    | 13.2 repository hygiene                       |
| Root-level CLI tests still mix several evidence scopes beside the composition root                              | 7.3 test topology                             |
| Canonical specifications still contain superseded default-route migration text until this delta is archived     | 13.6 archive and land                         |
| The work candidate and installed release both report `0.1.0-rc.110` although they are different product objects | 13.6 unique release identity before packaging |

## Decisions

### One Change, ordered semantic closures

All terminal work stays in this Change and Work Lane, but implementation moves
through ordered closures. Each closure starts with a failing observable
contract, changes one owner, runs focused proof, stages immediately, and runs a
heavy gate only after the closure is internally green. This preserves global
scope without mixing unrelated edits or repeatedly rediscovering state.

AIGW now has delivery priority over Proxy. Finish this product's remaining
semantic and quality closures, native artifact and client acceptance, signed
dual-Forge publication, and owned-residue cleanup before resuming Proxy work.
Proxy's existing signed source and installed service remain unchanged. This
sequencing does not reduce either product's terminal requirements.

The remaining execution order is dependency-driven. Task checkboxes remain the
only progress ledger; this table defines closure boundaries, not another status
store. A contradiction reopens its owning task instead of adding a parallel
mechanism or preserving an unsupported completion claim.

| Order | Existing tasks            | Closure and acceptance                                                                                                                                                                                                     |
| ----- | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | 7.2, 7.3, 7.8             | Consolidate semantic owners and behavioral evidence; delete duplicate fixtures, suffix-only families and forwarding layers. Preserve native platform selection and prove complete recovery at the owning boundary.         |
| 2     | 8.1, 8.5, 8.8, 8.10, 8.11 | Verify effective quality coverage for source, tests, tools, docs, schemas and configuration. Justify metric thresholds, exercise failure propagation, and remove custom checks superseded by native tools.                 |
| 3     | 9.7, 10.7–10.9            | Establish one signed update-proposal owner and object-preserving dual-Forge delivery. Recheck current stable dependencies and locks under 9.4–9.6; native Windows evidence need not wait for an unavailable GitLab runner. |
| 4     | 11.4–11.6, 11.8           | Validate command semantics and complete user journeys, review the real team profile, and inspect rendered documentation and terminal output. Formatting alone is not semantic or visual acceptance.                        |
| 5     | 12.1–12.9                 | Freeze one candidate and prove bootstrap, native artifacts, credentials, client integration, update, rollback, recovery, uninstall and measured performance on each supported platform.                                    |
| 6     | 13.1–13.7                 | Reconcile every requirement with evidence, remove remaining owned residue, archive and release a unique signed product object, align local and both Forge refs, then retire the merged proposal and lane.                  |

Documentation, naming and configuration changes accompany their owning repair;
they do not wait for order 4. Removal occurs in each closure, with order 6 as a
final audit rather than a backlog of deferred cleanup. New entities must reduce
caller knowledge or protect a named product invariant; a directory, wrapper or
policy file is not justified merely by a line-count limit.

One owner and one failing behavior are active at a time. Reuse current
observations instead of re-inventorying unchanged surfaces. Run the smallest
affected tests during repair, then one broad gate after fixture and consumer
coverage is reconciled. Report separately what is committed, verified locally,
published and installed; product preservation remains above cleanup speed.

Task descriptions retain their acceptance obligation and a concise current gap
or evidence pointer. Replace superseded notes rather than append repair history;
design owns rationale, Git owns implementation history, and raw verifier output
stays with its exact source. Compression must preserve task identity, order,
completion state and unresolved requirements.

Alternative considered: many small Changes. Rejected because the user's
cross-cutting terminal invariants would fragment, duplicate migration state,
and make deletion conditional on several overlapping branches.

### Accounts, Profiles, Routes, and Adapters are the complete control model

An Account owns endpoint capability. A Profile binds an Account, one admitted
client, one model, and one authentication owner: `account-token` means AIGW owns
the Account Token, while `client-native` means the selected client and its SDK
own credentials and request signing. A Route selects one Profile for one
client. An Adapter owns only discovery and transactional projection for that
client. There is no global Profile, `use --all`, implicit cross-client fallback,
or Provider-name behavior switch.

Configuration validation admits Account capability before Profile references.
Each Profile requires the endpoint selected by the existing admitted-client
protocol definition, not merely any endpoint on its Account. The same model
validation governs manifest import and local persistence, including unselected
Profiles; Tokens, installed clients and upstream availability remain separate
readiness concerns. Validation reads the supplied value directly rather than
cloning and normalizing every map before read-only checks. Profile checks form
one pass rather than separate label and relationship passes.

`setup --from` imports capability and may connect a chosen subset. `use` changes
one client Route. `sync` observes only AIGW-owned Tokens plus installed clients,
then converges eligible existing Routes and projections. `status` describes
local state; `check` probes AIGW-owned Account-Token Routes but only proves local
readiness for client-native Routes; `verify` is the sole live client-owned
authentication proof. `doctor` expands the same evidence without inventing
another state model.

The doctor report owns one typed outcome before either renderer runs. Its
diagnostic checks and canonical client observations jointly determine success;
deferred clients are allowed, but invalid, degraded, unavailable or unclassified
observations cannot accompany an overall success. Both output formats preserve
that result in their exit status and continuation. An already presented failure
does not enter the root command's generic error renderer a second time.

Recovery output separates execution mode from presentation. `sync` and `repair`
honor `--json` after preview or successful apply; only `--dry-run` suppresses
writes. Each existing command owns one result and continuation for both
renderers. Preview targets remain plans, not claims about applied targets.
The shared synchronizer retains commit and compensation authority; render
failure never rolls back a committed projection, and generic errors do not
invent transaction outcomes. No new recovery state machine or output framework
is needed.

The CLI root owns invocation output and finalization. Its existing writer
boundary captures human and JSON write failures without replacing the original
terminal used for width discovery. Parsed Cobra flags select generic error
format; the shared presentation Problem supplies both views. Once JSON output
starts, finalization does not append another result. Command and write errors
survive lock cleanup, and the executable entrypoint only selects an exit code
instead of rendering the same error again. Credential-helper stdout remains
reserved for credentials. Output state is scoped to one invocation, not retained
across repeated App execution.

Command help consumes Cobra metadata and pflag's native option usage rather than
reconstructing flag notation. Detailed descriptions and examples remain with
their commands; declared defaults are independent of parsed invocation values.
Inherited options retain their own section. pflag owns option wrapping and
indentation; presentation preserves that block without reflowing it. The complete command-tree test
verifies option projection and absence of configuration writes, while focused
fixtures cover detailed help, defaults and narrow output. No parallel command
catalogue or generated-document store is introduced.

Command argument admission precedes mutation locking. The root invokes Cobra's
required-flag and flag-group validation before acquiring the configuration lock,
because Cobra's normal execution order validates those relationships after
pre-run hooks. Commands validate their own argument shape through Cobra's Args
boundary: update admits candidate and checksum paths; setup admits manifest-mode
options; selection requires a Profile outside interactive mode; adapters admit
the client, executable and target arguments; creation admits identifiers and
client/model flags; configuration import admits its manifest path. Rejections
retain the native error and an actionable repair command. Existing operation and domain validators
remain in place, and valid mutations retain the same lock identity. Acceptance
starts with an absent configuration parent and proves that rejected shapes
leave it absent without invoking operations, credentials or client discovery.

Endpoint testing distinguishes an omitted selector from an explicitly empty
one. Omission tests selected Routes; empty or blank `--for` and `--profile`
values fail at argument admission instead of broadening the request to other
Routes. Cobra owns selector exclusivity. Admission diagnostics take precedence
over corrupt configuration, and rejected invocations perform no credential,
client or network operation. The admitted client registry also supplies help
text rather than another literal client list.

`use` validates a missing Token with the command context, then reuses credential
replacement and configuration commit as their existing transaction owners. A
failure before selection commits compensates the Token and any new automatic
backend choice, preserving newer state and reporting compensation errors. Output
failure after commit does not undo selection. Re-selecting an unchanged Route
with a newly supplied Token reports storage, not a no-op. Only unchanged
configuration with an already available Token qualifies for the no-op result;
client-native authentication requires no AIGW Token. First-time setup admission
remains distinct and is not weakened to reuse it for selection.

Service creation belongs to synchronization rather than the CLI. It admits
new Account and Profile identities and validates desired configuration before
requesting a Token. Existing Accounts are extended through Profile creation,
never implicitly replaced. Creation then shares selection's client-scoped
projection and credential compensation: its selected Route and client agree,
unrelated client edits remain untouched, and failures restore owned state while
preserving recovery errors. The CLI only acquires input and renders the result;
first-time setup retains its separate admission rule.

Account finalization intent and manifest Token ownership are checked by the
command argument owner before configuration locking. Invalid invocations name
the relevant command help and leave configuration storage absent. Cobra owns
pairwise exclusion between manifest setup and direct-profile options; direct
options remain composable. The later credential collector no longer duplicates
argument-shape validation.

`check` reports its observed scope in both human and JSON output. A configured
endpoint address is `endpoint_configured`, not endpoint health. A Route's
`check_passed` records only the checks required by its authentication mode:
client-native projection success remains `configured`; successful Account-Token
diagnostics refine it to `endpoint_checked`. Neither claims inference or
real-client execution. The unused `transport_ready` field and generic `ready`
state are removed, with no compatibility alias. `next_action` alone carries
repair or verification continuations, replacing the duplicate `fix` projection.

Optional Provider diagnostic credentials do not participate in that decision
and are not read for display-only balance suggestions. `account connect` and
`balance` retain their explicit diagnostic responsibilities. This removes the
former human-only dependency that could reject a healthy Route while JSON
reported success; the CLI regression measures both results and zero diagnostic
credential reads against the same working Route.

Claude projection readiness belongs to the settings owner. Local inspection
and live-verification planning share its read-only synchronized-settings
validation, so executable discovery cannot mask missing or drifted settings.
The same decision reaches `status`, `check`, and `doctor`; none repairs the
projection as a side effect of inspection.

Alternative considered: retain a global default for convenience. Rejected
because one Profile cannot truthfully select models for clients with different
protocols and configuration surfaces.

Codex selection projection uses the locked native TOML parser to identify root
assignments. Full-document regular expressions previously confused named-profile
keys with root selections and interpreted dollar signs as replacement variables.
One selection owner now supplies exact source ranges to projection, inspection,
catalogue admission and withdrawal. Quoted keys, multiline original values and
comments are preserved; native string encoding retains literal model/path values
and rejects invalid UTF-8 instead of silently replacing bytes. State attribution
is decoded once at its existing owner, not redundantly before reconciliation.
No sidecar schema or installed-client state changes are required.

Scheduler table boundaries use the same native parser rather than a separate
line scanner. The table owner delegates body assignment edits to the selection
owner and decodes integer values with the native TOML decoder. This admits quoted
headers and keys, comments, signed values, hexadecimal values and numeric
separators while preserving multiline instruction strings and neighboring
profiles. Withdrawal emits the recorded scheduler values directly; the former
document-wide ownership-comment cleanup is removed. Scheduler snapshots record
values, not original formatting, so only unrelated source is byte-preserved.

Native tests cover complete projection/validation/withdrawal, nested catalogue
ownership, malformed input before writes, and source-preserving edits. A Go
benchmark uses the same 100-profile document and exact baseline source overlay;
it measures in-memory projection only, not filesystem, credential or client
latency. Runtime and final cross-platform release evidence remain separate.

### Credentials use one selected backend and typed purposes

Backend selection is an installation fact. Native keyrings are admitted only
after bounded non-interactive capability observation; the environment backend
is explicit and read-only; a platform-safe file fallback is selected only by
the documented automatic policy. API Tokens and Provider diagnostic credentials
use distinct typed slots in the same backend. No command searches several
stores or opens a prompt for read-only status.

Alternative considered: search native, file, and environment stores on every
read. Rejected because it creates several authorities, unpredictable prompts,
and platform-dependent results.

### External gateways remain ordinary endpoints

AIGW may project any admitted HTTP endpoint. It neither identifies nor owns the
product behind an endpoint and does not install, inspect, start, stop, update,
or uninstall it. A team manifest carries the endpoint chosen by the team;
selecting a loopback gateway is an operator-owned Account choice, not an AIGW
default.

Alternative considered: bundle or automatically configure an external gateway. Rejected
because it violates the control-plane/data-plane boundary and prevents each
product from being useful independently.

### Extensions narrow as they approach the core

Ordinary Providers are declarative. Only protocol-specific request validation
or Provider-native diagnostics may add a leaf Adapter. Cloud credentials and
request signing remain client-native whenever the admitted client already owns
that capability; AIGW does not duplicate an SDK credential chain or signer.
New clients implement a small contract for discovery, projection, credential
binding, status, rollback, verification, and uninstall, then pass the shared
conformance suite. Existing client or Provider code is not modified unless the
common contract itself changes.

Library admission follows the durable
[dependency policy](../../../docs/governance/change-and-release-policy.md#dependency-and-framework-admission).
This Change retains the existing library boundaries pending a demonstrated
replacement benefit. The review below records scope and the proof a replacement
needs; it is not a benchmark or a claim that alternatives cannot meet it.

| Responsibility               | Retained implementation                                   | Replacement question                                                                                                            |
| ---------------------------- | --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| CLI and terminal interaction | Cobra, pflag, Huh, Lip Gloss                              | Does another framework remove parsing or presentation work while preserving command grammar and accessibility?                  |
| Configuration                | `go-toml/v2` and the Account/Profile/Route domain         | Can Viper, Koanf, or schema tooling reduce validation and persistence work without a second configuration authority?            |
| Tokens                       | `go-keyring` and the selected portable backend            | Can a backend library preserve non-interactive selection, Unix file invariants, and Windows DPAPI with less owned code?         |
| Program update               | Shared durable staging and bounded `robustio` operations  | Can an installer or package manager simplify replacement, verification, rollback, recovery, and exact cleanup together?         |
| Release                      | GoReleaser, Syft, OSV-Scanner, OpenSSH, and release tools | Can native pipes or another release tool replace final-matrix evidence or object-preserving peer publication?                   |
| Provider diagnostics         | Standard HTTP and protocol-specific leaves                | Does an SDK remove actual request or credential complexity? An HTTP server framework does not match this control-plane product. |
| Tests                        | Go testing, scoped fixtures, and static analyzers         | Can a test library delete repeated setup or improve assertions without adding another test runner?                              |

Release-tool evaluation must include construction, final-matrix verification,
readiness, and independent GitHub/GitLab publication. Signing must cover the
complete archive, vulnerability, license, and provenance outputs; publication
must preserve one product object and support create-or-verify retries on either
peer. GoReleaser native pipes, `ko`, nFPM, JReleaser, Cosign, and hosted
attestations remain evaluation candidates only where they replace one of those
responsibilities. No additional dependency is admitted by this review, and no
unmeasured source-line count is used as evidence.

OSV-Scanner owns vulnerability detection. Release normalization preserves its
advisory identity while restricting `fixed_versions` to the reported package's
ecosystem and name, including an explicit ecosystem wildcard, and to SEMVER or
ECOSYSTEM ranges. Git commits, another package's fixes and unattributed ranges
are not package releases. The wire adapter owns this selection and stable
deduplication; the report writer does not interpret nested range events. These
values are advisory-reported fixes, not a computed upgrade recommendation or a
new vulnerability matcher.

### Semantic topology precedes physical movement

Follow the architecture's [module-depth contract](../../../docs/architecture/authority-and-projection-boundary.md#module-depth).

The package map is derived from product responsibilities and dependency
direction. Only then are flat suffix families, concatenated names, ambiguous
buckets, tests, tools, and documents moved. A move must reduce owners or make an
enforced boundary visible; compatibility packages and re-exports are forbidden.

This review covers the whole repository, including root metadata, configuration,
manifests, documentation, active and archived OpenSpec, generated CI, source,
tests, and repository tools. For each responsibility, inspect its consumer,
dependency direction, change reason, lifetime, and navigation. Architecture
classification establishes declared membership, not semantic correctness. Keep
ecosystem-required locations, useful package-local tests, and native platform
selection; do not turn every concern into a directory or package. Generated
carriers point to their owner rather than acquiring an independent policy.

The root/configuration ownership review covers all 19 tracked root files and
nine `.config` files at source `d2feaa4f`. Root files serve native discovery,
dependency closure, product identity, legal metadata or reader entrypoints;
the existing authority map records these responsibilities without a second
file inventory. `.config` contains policy data, not implementation. The optional
code indexer's `.cbmignore` is classified as development tooling, not a quality
gate. Seven Taplo default repetitions are removed: both configurations produce
byte-identical output for all ten tracked TOML carriers, including `mise.lock`.
Native cross-checkout conformance additionally protects authored key order,
array order and multiline grouping. Full rule coverage, hosted hook behavior,
dependency freshness and whole-repository topology remain separate tasks.

Optional provider-account diagnostic results belong under
`internal/providers/diagnostic`, not a top-level Account domain. The result
contract remains independent of the provider registry and concrete adapters,
so both depend on it without a cycle. Its fields and consumers are unchanged;
the former package and its unused CLI import allowance are removed without a
compatibility alias. Canonical Accounts remain owned by configuration.

Production import allowances are checked against the union of native Go package
imports for all six declared OS/architecture targets, not one host's import
graph. Eleven unused allowances are removed, including an inapplicable owner
entry for test-only upgrade acceptance. The gate governs production edges;
test-only references cannot justify dormant production coupling. Each retained
edge has a current cross-platform consumer, and existing source validation still
requires explicit admission before a new dependency is introduced.

Native duplicate and unused-parameter analysis informs deletion, not automatic
rewriting. Codex tests call the production projection directly; no test-only
production wrapper remains. The entrypoint ignores executable aliases by
construction. Fixture forwarding and fixed-choice parameters are removed where
they hide no behavior; duplicate provider failures share their setup and retain
both assertions. Explicit archive/version inputs remain part of fixture meaning,
and host-dependent parameters retain their cross-platform role. A single-host
constant-propagation finding does not justify deleting another platform's path.

Architecture membership declarations share one root, cardinality, uniqueness
and member-name validation path. Their differences remain explicit: child and
composition lists are nonempty, peer and import allowances may be empty, and
composition members are Go file base names. All reuse the existing portable
relative-path predicate; four diverging loops and an error-only wrapper are
removed. Root ordering makes membership diagnostics deterministic. Real
regressions cover names previously accepted despite path or whitespace ambiguity;
existing topology and empty-allowance behavior remains covered. Native scores
for the membership validator fall from 32 to 12 cyclomatic complexity and from
45 to 20 cognitive complexity without introducing another policy framework.

Credential-helper tests separate matching projections from stale bindings.
Token rotation, model changes and label changes must still return the selected
Account Token; changed client, Account or endpoint identity must fail before
secret observation and leave stdout empty. These two outcomes no longer share
conditional assertions or setup for a secret store they never consume. The
existing test owners and real Store fixture remain; no helper layer or product
behavior is added. Native Go overlays verify that both predecessor and current
tests detect omitted binding checks, model-bound fingerprints and corrupted
Token output. Statement coverage stays complete, while the largest affected
test's cognitive score drops from 62 to 40 and the test file becomes smaller.

Client admission order is the sole lifecycle order. The registry retains that
ordered ID sequence and its ID-to-adapter lookup, not a second adapter sequence
derived from constructor argument order. Discovery, planning, application and
reverse compensation therefore agree even when implementations are supplied in
a different order. Explicit client selection retains the caller's requested
order. The existing compensation test exercises reversed constructor input;
the unused admission-record map is removed rather than synchronized separately.

Ordinary configuration commits derive an ordered affected-client set from the
registry rather than collapsing it to a global boolean. The transaction passes
that set through preflight and apply; an empty set persists configuration only.
Previously, changing either protocol endpoint could reset the other client's
external model preference. Real-file regressions prove both directions now
preserve the unrelated projection and sidecar byte-for-byte while committing
the selected endpoint. Shared edits retain every affected client in admission
order; returned scope slices do not expose registry state. Explicit sync/repair
continues to reconcile the full requested scope. The synchronization boolean
forwarder and CLI's repeated outcome prediction are removed; profile editing
reports saved metadata, and acceptance checks the actual projected label.

Registry application owns compensation to completion. Its product caller needs
only success or failure, so it returns an error rather than an aggregate
rollback handle used only by tests. Adapter receipts remain internal to the
transaction; an unchanged adapter returns no receipt. On failure, the registry
attempts every prior receipt in reverse order and joins rollback errors without
discarding their identities. One failed cleanup cannot suppress an independent
client’s cleanup. Success retains no caller-managed compensation obligation.

Cancellation admission belongs to those same transaction owners. Configuration
commit rechecks after snapshot capture and projection preparation; the registry
checks before planning and before each adapter starts. Cancellation between
adapters follows the existing reverse compensation path, preserving cancellation
and recovery conflicts without a new state machine. Real-file regressions prove
configuration, backup, checkpoint and client preservation; ordered adapter
tests prove no later write and independent compensation. A completed final
adapter remains committed rather than being retroactively failed by cancellation.

Codex compensation follows the same contract at its artifact boundary: retain
each original error with its target path and join failures with the standard
library, rather than flattening them into text. The receipt retains only the
applied artifacts needed for compensation; preview plans and the returned
transaction-ID copy had no consumer and are removed. Sidecar identity remains
unchanged. A real-file regression preserves a later user edit, restores the
other target, removes owned sidecars and retains the exact conflict cause;
write-failure injection also preserves both commit and compensation errors.

The existing `transaction.FileSnapshot` owns construction and exact equality
for configuration persistence, Codex projection and guarded filesystem writes.
Its copied bytes, digest, existence and permission bits have one implementation;
domain-local constructors and comparators are deleted. An absent file remains
distinct from an existing empty file. Secure credential storage retains its
separate constant-time value and filesystem-identity checks: that is a stronger
security boundary, not another spelling of configuration equality.

Compensation is exhaustive within each owner. Claude restores settings and its
sidecar independently; configuration persistence restores config, backup and
checkpoint independently. A postimage conflict preserves the newer file,
continues safe restoration of the others and joins every original error.
Checkpoint-invalidation failure follows that same rule for its already-written
config and backup. Restored checkpoint bytes remain evidence of their captured
configuration, never a verification claim about a newer external configuration.
Real-file conflict tests cover both single and simultaneous recovery conflicts.

Credential replacement closes an owned staging handle before removal on failure
and reports the exact staging name together with the original error. Once the
rename commits, the old staging name is no longer owned and is not removed.
Explicit client verification owns a private working directory, not a loose file
in a shared temporary directory; its cleanup covers client-created output and
retains invocation failures. The coverage command similarly reports exact-file
cleanup failure and returns a failing status without hiding a test failure.
Its single-file ownership does not authorize recursive deletion.

The module-depth review retains these distinct boundaries:

| Intent                                    | Owner                               | Why the boundary remains                                                                                                                               |
| ----------------------------------------- | ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Setup, selection and configuration commit | `internal/synchronization`          | Owns credential compensation, client scope, preflight, persistence and recovery; the CLI supplies intent and renders results.                          |
| Client composition                        | `internal/client`                   | Selects admitted adapters and compensates them in reverse order; individual client packages own their artifact transactions.                           |
| Settings and model catalogue projection   | `internal/claude`, `internal/codex` | Preserve client-specific ownership and restoration semantics without exposing internal file writes to callers.                                         |
| Credential durability                     | `internal/secrets`                  | Owns backend selection, secure file identity, staging and compensation; stronger credential guards are not duplicated configuration semantics.         |
| Live verification                         | `internal/verification`             | Consumes synchronized settings, executes a bounded request and owns its output; catalogue membership remains a separate claim.                         |
| Program update                            | `internal/upgrade`                  | Owns peer selection, downloads, candidate admission and replacement; archive structure has its own pure owner.                                         |
| Repository verification and publication   | Existing `tools` packages           | Native tools own their analyses; repository commands own declared inputs and outputs. CUE projects CI, while Forge transport preserves signed objects. |

These boundaries are supported by existing real-file, process and signed-Git
regressions, not by counting interfaces. The shared atomic writer owns its
temporary pathname; its commit helper owns exactly one handle close on every
path. Write and close failures remain inspectable together. Failed writes report
exact-file cleanup errors, while successful renames relinquish the old staging
name. Stage-fault tests and real-file replacement tests verify these boundaries
without adding a filesystem framework or changing callers.
Complete topology, quantitative quality, hosted admission and final product
journeys retain their separate tasks; this review does not certify them.

Daily selection belongs to the same synchronization owner as setup and repair.
`SelectProfile` derives scope from the Profile, owns optional validated Token
storage and compensation, discovers that client and commits only its projection.
The shared transaction forwards this scope through both preflight and apply.
The CLI-owned rollback, configuration comparison and commit coordination are
removed. Repeated selection remains a scoped reconciliation with no config or
checkpoint rewrite. Both real-file acceptance directions deliberately drift the
unselected client's projection while proving the selected projection changes;
an unrelated client's conflict is not a selection failure.

Authenticated endpoint requests have one credential-domain execution boundary.
Validation, connectivity tests and readiness diagnostics use the same native
HTTP redirect protection. Validation and connectivity additionally share
bounded response lifetime and closure, removing the CLI's duplicated request
mechanics. The response status remains an observation: each consumer owns its
narrow interpretation, and a redirect is never healthy authentication.
The connectivity result carrier lives beside connectivity behavior, not under
the unrelated read-only status renderer.

Provider-account diagnostics share the credential HTTP contract directly,
removing their duplicate interfaces and forwarding adapter. Native-client
redirect protection covers both account and Token endpoints. The provider
reader admits one complete JSON document within the existing response budget
and preserves read and close errors; a full final search page is incomplete
evidence, not Token absence. Real HTTP regressions cover redirect isolation;
body-lifetime and pagination tests cover partial observations. Registry
acceptance verifies successful dispatch and report values rather than a
source-AST pattern or one of several unrelated fixture failures.

The upgrade review found source errors, Forge authentication and downloads,
installation helpers, and generic transport tests mixed in catch-all files.
It also found duplicate installation scenarios and an unconfigured-source test
that inherited a configured GitLab source, so an unrelated missing helper could
make it pass. These findings invalidate the earlier global deduplication
completion claim: task 7.5 is reopened. Repairs below are bounded progress;
tasks 7.2, 7.5, 8.1 and 8.11 remain open for complete topology and effective-check
coverage. Completed behavioral ownership and module-depth reviews do not close
those remaining obligations.

Publication now separates transport from release orchestration inside the
existing package. HTTP authority, redirect handling and response consumption
belong together; their cross-Forge tests no longer live under a GitLab filename
or among release-construction failures. Provider payloads remain beside the
publication operations that consume them. This redistribution retains five Go
files and preserves all 71 declarations, including 39 tests, by canonical AST
comparison. It adds no package, forwarding entrypoint or behavioral variant.
The native 500-line trial no longer flags this owner; that is bounded topology
progress, not repository-wide ELOC enforcement.

Credential storage repeated the default API-Token view on five backends
alongside a second purpose-aware interface. The existing credential-kind view
now owns all four consumer operations and validates input before backend
resolution. Private backends implement only purpose-aware storage. Regressions
reproduced rejected repeated purpose selection and backend probes for invalid
Account reads and presence checks. Purpose selection is now idempotent and
retains backend identity, observation, read-only behavior and compensation.
Twenty duplicate entrypoints and their runtime interface conversion are removed;
platform protection and persistent slot identities remain unchanged. No backend
implementation is exported and no forwarding package is added. This supports
the completed behavioral ownership and module-depth reviews; the global consumer
audit remains under task 7.5.

The tracked-root review also exposes unresolved boundaries, not a completed
directory audit:

| Surface                         | Current finding                                                                                                                                                                                                                                                                                                                                         | Closure owner              |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------- |
| Product and tests               | Behavioral ownership and module-depth reviews are complete. Private access and native platform selection remain intentional; complete physical topology still requires review.                                                                                                                                                                          | 7.2                        |
| Repository tools                | Native artifact journeys now belong to release, not CI dispatch. The shared quality sequence still consumes `tools/repository/protected_lifecycle.go`; ETHOS must provide equivalent detached-checkout admission before that generic overlap can be removed.                                                                                            | 7.2, 7.5, 8.11             |
| Root metadata and configuration | Editor defaults now inherit once; Git applies LF to all detected text rather than an incomplete extension list. Ignores follow owned output directories instead of hiding arbitrary test/output suffixes. Prettier applies locked defaults; EditorConfig owns editor defaults and its independent check. Neither has a redundant concern-local carrier. | 7.6                        |
| Documentation and manifests     | The documentation root is connected, but audience placement, rendered tables and diagrams, current model claims and team-profile consumption still need direct acceptance. A release-coordinate example is not a deployed team configuration.                                                                                                           | 11.4–11.8                  |
| CI, OpenSpec and outputs        | CUE is the declared projection owner; actual event conformance and output cleanup are separate proof obligations. Archived Changes retain historical intent, not current implementation authority.                                                                                                                                                      | 8.10, 10.7–10.9, 13.1–13.2 |

Upgrade transport review reproduced four defects hidden by the original direct
redirect tests: replacing the native redirect policy removed its request bound,
origin comparison ignored the scheme, credentials reappeared after returning
from a foreign origin, and an intermediate HTTPS hop could downgrade when the
first request was HTTP. The existing updater now composes the native
ten-request default with the complete redirect-chain credential and TLS
boundary. Real HTTP/TLS chain tests and focused callback tests establish these
cases; this does not establish installed-product or native-platform acceptance.

Formatting now uses the shared checkout-bound Git inventory rather than CLI
globs or `.gitignore` filtering, which silently omitted tracked ignored files.
The package script and quality graph invoke the same executor. Its native
Prettier adapter selects supported formats and applies `.prettierignore` only
to official OpenSpec archive history; `.gitignore` still owns untracked output.
Missing inputs and a scope with no supported authored files fail. Native
conformance covers current Markdown, JSON and YAML, tracked ignored files,
literal glob characters, checkout paths with spaces and parent configuration
isolation. Paths travel relative to the requested checkout, so filesystem aliases
cannot move authored files outside native ignore matching. No custom formatting or ignore parser is introduced.

All three Node check adapters consume the streamed inventory through native
`node:stream/consumers`, not a synchronous read of a potentially incomplete
pipe. Fragmented input larger than one megabyte exercises formatting, Markdown
and Mermaid checks without command-line limits or retry machinery.

Repository inventories bind both the working tree and index to their explicitly
requested checkout. The CI and architecture scanners remove only an inherited
`GIT_INDEX_FILE` override from their Git subprocesses; ordinary Git operations,
configuration, signing and transport retain their caller environment. Real
two-repository regressions prove that a foreign index cannot silently omit a
tracked file, including tracked paths matched by an ignore rule. CI retains its
current tracked-and-untracked scope; architecture retains its tracked-only scope.

Prettier uses locked defaults through its native API without configuration
discovery; its redundant configuration is deleted rather than relocated.
EditorConfig owns editor defaults and a separate native check, not Prettier
options. Conformance proves isolation, current-carrier coverage and unchanged
source bytes. Root ownership is complete under task 7.6;
effective-check scope remains open under task 8.1. The unconsumed
Forge environment template is removed: build inputs already belong to the
release constructor and protected execution context. Contributor setup and
verification now have their own section, rather than being nested beneath
publication; duplicate coverage wording is replaced with its canonical policy
link. Documentation navigation remains
owned by `docs/README.md`; its existence does not prove every linked document
belongs in its current audience or responsibility category.

Local link verification now enables lychee's native heading-anchor checks.
The prior command admitted a link to an existing document even when its
fragment did not exist. Real command conformance admits an existing heading,
rejects a missing heading, and verifies source preservation. This adds no link
parser and does not turn offline source checks into external or rendered proof.

CI projection has one owner under `tools/ci/projection`. The CLI supplies the
repository root and check mode; the package renders every CUE output before
reconciling its private fixed artifact set. Callers do not coordinate rendering
and file writes or provide arbitrary output paths. Graph and projection
contracts live with that owner; CLI tests retain argument-routing coverage.

Portable upgrade artifacts have one owner under `internal/upgrade/artifact`:
platform layout, manifest verification, and executable extraction. The updater
owns release selection, candidate startup verification, replacement, and
rollback. The artifact owner has no updater dependency. Manifest verification
returns the measured digest, so peer comparison does not hash the same download
again. Strict SemVer parsing and precedence use the maintained Masterminds
library; AIGW only normalizes the release-tag prefix at its input boundary.
Archive tests live with that owner; installation and transport tests retain
their respective behavior. No forwarding package preserves the old layout.

An online update owns one temporary workspace for all peer downloads and removes
it through the existing cross-platform filesystem library. Download helpers
receive that workspace rather than returning cleanup callbacks or coordinating
independent resource lists. Failed-peer directories remain inside the same
owner's lifetime. Errors retain their original causes, including cleanup errors;
cleanup after successful replacement reports that the program was updated rather
than implying rollback. Both peers classify transport unavailability consistently,
while authentication and integrity failures remain terminal. Real-file tests
verify workspace removal and preservation of unrelated files, current program
bytes and the rollback copy; native macOS failure injection covers cleanup denial.

Release construction follows the same ownership principle without sharing a
product transaction abstraction. Construction and native acceptance report exact
workspace cleanup failures. Replacing an existing output uses a uniquely created
private sibling backup; a similarly named operator file grants no deletion
authority. Failed publication restores the previous output; failed restoration
retains its exact backup path and both causes. Successful publication followed by
cleanup failure reports the new output as published. This is recoverable
directory replacement, not a claim of uninterrupted atomic visibility across
two filesystem renames.

Program update and operator-requested rollback share one recoverable replacement
boundary, including predecessor retention, file mode and restoration errors.
Cancellation is admitted before preparation and checked again immediately before
replacement; after the first rename begins, it completes or compensates
without interruption. A completed swap is not reported as canceled afterward.

Native Windows release acceptance exposed a sharing violation while renaming the
installed executable. The former updater library directly called `os.Rename` and
deleted the previous program first; retrying its whole transaction could repeat
those effects. Its narrow replacement is now composed from existing durable-file
staging and `robustio.Rename`, with one compensation owner shared by update and
rollback. The unused updater dependency is removed. Candidate cleanup belongs to
the replacement workspace on success and failure. Real Windows handle tests
exercise temporary source/destination locks and persistent source locks; a
portable fault-injected rename test verifies activation compensation and retained
causes. macOS immutable-file testing rejects leftover staging, and a portable
foreign-directory case replaces tests coupled to the old library's staging name.
Native artifact acceptance remains required: cross-compilation and a passing
macOS test cannot establish Windows behavior.
Remove the unused one-value distribution Channel and its defaulting test.
Rollback acceptance covers reversible bytes, operator guidance and the complete
owned directory contents in one scenario, rather than duplicate swaps and
historical filename prohibitions. Canceled rollback preserves both program files.

Package-private upgrade tests follow source selection, GitHub, GitLab,
installation, candidate admission, and updater orchestration. The mixed
transport and validation files are absorbed into those existing owners. Public
acceptance lives under `internal/upgrade/acceptance`, in a separate native test
binary. This is a fixture-lifetime boundary, not a directory for every test
category: simulated release configuration can no longer configure private
tests. Private transport tests clear inherited credentials; acceptance owns
its isolated client home and reports cleanup failure. No product API or
exported test framework is added. The acceptance files group candidate,
installation, source selection, GitHub, GitLab and peer agreement; GitLab HTTP
and redirect scenarios no longer masquerade as installation tests.
Local-candidate acceptance
owns successful replacement and exact version reporting; the Windows candidate
case owns executable-name admission. Delete their redundant narrower scenarios
while retaining checksum-before-extraction, rollback, and native OS tests.
Neither archive inspection nor a synthetic version runner proves native Windows
execution. The unconfigured-source case clears both Forge coordinates and
asserts that exact error with an isolated executable search path.

CLI acceptance groups evidence by the user operation, not by the file where a
regression was first added. Manifest import owns conflict and credential
reporting; route selection owns activation; endpoint checks and native-client
verification remain distinct. Configuration recovery stays separate from
program update and rollback. Account diagnostics and catalogue discovery each
have one test owner. Shared isolated application, discovery, prompt and manifest
fixtures live together; scenario-only helpers stay beside their consumers.
Relocation preserves test declarations and assertions; it does not justify a
new exported fixture framework or claim that product behavior has changed.

The completed test-ownership review keeps evidence boundaries explicit:

| Responsibility       | Test owner                                   | Placement rationale                                                                                                                                                                            |
| -------------------- | -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Public CLI journeys  | `internal/cli/acceptance`                    | One isolated application fixture; operation-named files omit the redundant command suffix. Guided and explicit setup share setup ownership; manifest setup remains a distinct journey.         |
| Credential semantics | `internal/secrets`                           | Store faults share real memory state; typed slots, backend inspection, persisted selection, file durability and native credential observation retain distinct contracts.                       |
| Client composition   | `internal/client`                            | Registry ordering and compensation differ from future-adapter conformance and built-in integration; their fixtures must not collapse into one configurable fake.                               |
| Claude projection    | `internal/claude`                            | Public verification plans remain external-package tests; private real-file transactions retain access to their compensation seams.                                                             |
| Codex projection     | `internal/codex`                             | Public configuration/provider tests remain external. Private transformation, read-only inspection, scheduling and reconciliation have separate owners rather than generic supplementary files. |
| Release update       | `internal/upgrade`, `artifact`, `acceptance` | Archive admission, private transport and process-isolated public journeys have different lifetimes and visibility. Forge-only fixtures stay with their scenarios.                              |

Native OS suffixes and build constraints are executable selection, not naming
clutter. Keep Unix-only credential tests Unix-only and portable tests portable.
Package-local tests do not justify a new exported test-support package. Test
names state their real backend: replacing an in-memory Token is not evidence
of overriding a read-only environment Token. Consolidation preserves test
bodies and supported platform selection, not merely test counts.

Command composition, help rendering and completion belong beside the CLI root,
without constructing credential, network or discovery fixtures. The rendered
group order and titles must consume Cobra's declared groups, not a second
hard-coded category list. Keep one ordered public-help journey assertion and
the narrow-terminal width contract. Configuration command admission is the
positive `export`, `import`, `path` tree, not an inventory of retired names.
Update, account-write and response-body fixtures belong beside their only
scenario consumers; shared fixtures retain only genuinely shared capabilities.
Remove the forwarding test executor and call the public CLI entrypoint directly.
Configuration locking starts only after Cobra resolves a valid executable
command and its flags. Help and completion rendering leave configuration storage
untouched; dry-run classification consumes the same parsed flag value as the
operation, rather than a second argument parser. Removed commands have no lock
policy. This distinguishes a command's capability from an invocation's effect.

Client tests own projection-change and credential-binding decisions. The
synchronization suite owns transaction preparation, cancellation, persistence
and compensation; client-native projection remains a transaction scenario, not
a separate model-provider test file. Its persistence double implements only the
consumed snapshot, commit and restore contract, without simulating the store's
private write sequence. Real persistence failure coverage stays with the
configuration owner. Duplicate runner and credential doubles are consolidated.

#### Migration boundaries

Topology migration follows semantic operations, not suffix matching. Review
`internal/secrets` by backend selection, credential storage and transaction
ownership; `internal/upgrade` by source discovery, candidate admission and
installation; CLI acceptance by user journey. These are review boundaries, not
a predetermined demand for new packages. Preserve Go platform suffixes where
they select real platform implementations and co-located tests where they
exercise one package contract. Split only a genuinely distinct dependency or
invariant boundary, absorb single-use wrappers, and delete obsolete consumers
in the same closure. Product behavior, readable dependencies, and fewer
parallel owners establish success, not directory depth.

The process package keeps native implementation and test selection together.
Pipe-drain deadlines and inherited descriptors now live in `pipe_drain` test
carriers, with Unix and Windows variants and one shared controlled-deadline
fixture. The Unix shell precondition stays with ordinary Unix runner tests,
which also consume it. These moves preserve test bodies and platform tags;
another exported process-test package would weaken access to the private owner.
Architecture package documentation moves to its command entrypoint rather than
describing the whole package from a decision-record implementation file.

Native installation, credential and upgrade journeys move from `tools/ci` to
the existing `tools/release` owner. CI command tests retain only their own file
and checkout helpers, so they no longer depend on product-lifecycle fixtures.
Release construction and contributor commands select the moved tests directly;
the CUE entrypoint remains unchanged. No test-support package or alternate runner
is introduced. Source-archive root derivation remains independent of Git state.

That boundary review exposed divergent `VERSION` readers: CI admitted arbitrary
single-token text while construction required strict SemVer. The construction
reader moves to `tools/release/readiness`, where CI, construction and native
journeys reuse it. Public native admission now rejects malformed identities
before invoking any downstream command. The permissive reader and its duplicate
unit tests are removed; strict reader tests live beside the owner, while CI and
construction retain integration checks. Version validity remains distinct from
GA signing readiness.

Credential selection is one private responsibility within `internal/secrets`:
`selection.go` owns selection policy, secret-free observation, the persisted
choice, and its compensation. `store.go` retains the storage contract rather
than mixing in that lifecycle. Credential-kind views preserve the same backend
identity for observation and rollback. They do not create another backend or
make callers recover selection files. This consolidation replaces the separate
`backend_choice.go`; complete physical topology and consumer review remain open.

Automatic selection uses the same admission path as explicit selection and
retains the admitted Store. It must not probe a temporary keyring and then
probe a replacement instance: one resolution has one capability observation.
Only an absent persisted choice permits initial secure-file fallback; an
unavailable explicitly selected or persisted keyring remains an error. Repeated
inspection uses that resolved backend without writing credential storage. The
portable policy test covers available keyring and file fallback for all three
supported platforms; it does not replace native credential acceptance.

Backend persistence is not cached independently of its file. Each inspection
reads current choice metadata while retaining the invocation's admitted backend:
matching metadata is persisted, absence is deferred, and conflicting or invalid
metadata is unavailable. This removes the duplicate persistence flag and its
updates across selection, mutation and compensation. Inspection never switches
backends or reads credential values; existing mutation and compensation guards
remain responsible for writes.

Diagnostic credentials, their JSON schema and the typed storage adapter belong
to `internal/secrets`; `internal/providers/diagnostic` owns diagnostic results. The
constructor binds diagnostic slots on the already selected backend, replacing
caller-provided error classifiers and the parallel diagnostic memory store.
CLI fixtures use that same adapter over their API Token backend. The shared
recording fixture observes either credential type without reimplementing its
storage. Constructor, interface and result names denote credential storage,
not a diagnostic operation. Slot presence, readable valid content and Provider
authentication are distinct guarantees; stored incomplete data is not absence.
Environment pair decoding uses the same schema, and existing scoped/native
backend tests retain their separate evidence boundaries.

Client credential rotation follows the [synchronization ownership
contract](../../../docs/architecture/authority-and-projection-boundary.md#synchronization-and-setup).
The registry's binding operation returns its outcome instead of requiring a
separate Account-use query. It reports a refresh only for configured native
storage targets, not helper-based clients or targetless adapters. Synchronization
retains compensation without a public forwarding method. No parallel state model
is added.

Repair follows the same result boundary: the transaction returns whether native
bindings were updated, and the CLI renders that outcome instead of recomputing
intent and calling it authentication. Targetless adapters require no binding
refresh. Read-only binding comparison remains for rename previews, not as an
extra prerequisite or completion claim in the execution path.

Upgrade delegates helper execution to `internal/process.Runner`, including
streamed assets. In-memory and file output share platform launch, child
ownership, diagnostic limits, and bounded pipe draining. The upgrade-only
executor and silent-truncation writer are removed. Execution conformance lives
with the process owner; upgrade tests retain Forge selection, authentication,
artifact admission, installation, and rollback responsibilities.

Verification and every supplying layer use `process.CaptureRunner` directly.
The three repeated `Run` interfaces and late capture assertions are removed;
capture-only fixtures exercise the existing client journeys without unused
methods. Consumer review found no production use of the uncaptured `Run`
operation or process replacement. Their Unix/Windows implementations, mode
flag and exclusive tests are deleted. Captured child ownership, diagnostics,
explicit environments, Windows batch invocation and file streaming retain
their behavioral tests; removing an unused mode does not remove those contracts.

Private upgrade tests share two process fixtures in their existing fixture
owner: capture-only observation and file-capable execution. Both retain full
process plans; file execution distinguishes absent output, empty output,
successful payload and write failure. The earlier per-error runners, custom
not-found wrappers, duplicate call lists and writer-only subclasses are removed.
Native `exec.ErrNotFound` and ordinary wrapping exercise unavailability without
recreating standard errors. Protocol-specific GitHub sequencing and native
platform locking fixtures remain separate because they prove different
contracts. Existing upgrade test functions and assertions remain intact.

Credential test faults wrap the existing in-memory Store; they do not own a
second key/value implementation. Replacement, diagnostic errors and unavailable
capability cases share that wrapper. Keyring availability tests invoke the real
metadata-observation boundary with an injected observer, covering present,
absent and unavailable results without accessing the host vault. Filesystem
identity, durability and recovery fixtures remain beside their private owners.

Public upgrade acceptance shares one release runner with explicit download
outcomes and one archive constructor, including repeated-entry admission.
Missing executables, GitHub release sequencing and GitLab streamed assets retain
separate capabilities; Forge-specific fixtures live beside their consumers.
Native slice operations replace duplicate command-match loops and forwarding
helpers. Client projection keeps distinct registry-compensation and future-
adapter lifecycle fixtures: combining them would hide different contracts.
Private Codex identity and real-file projection tests remain package-local;
native platform suffixes continue to select platform evidence.

Windows path derivation appends owned components to an explicit base; it is
not a general path-normalization API. The old joiner stripped the leading UNC
or extended-length namespace and rebuilt a single root separator, changing
the selected location. A narrower append operation preserves that prefix and
normalizes ordinary separator spelling without touching the filesystem. The
shared regression covers configuration, data, credentials, Claude settings and
installation paths across drive, rooted, UNC and extended-length bases. On
Windows it additionally compares each result with native `filepath.Join`.
This proves path derivation, not access permission or lifecycle behavior on an
actual network share. No parallel Windows path library or dependency is added.

Configuration integrity is enforced at the value and persistence boundaries.
`Config.Clone` isolates nested Account diagnostics, maps and Adapter targets;
read-only runtime resolution does not clone the model. Tests assert value
independence, unchanged query input and the selected endpoint rather than
historical helper names.

The Store admits exactly one complete checkpoint JSON document. One validator
serves reading and writing: the client scope must be a nonempty, unique subset
of admitted IDs. Invalid input cannot replace a usable checkpoint.

Verified backup capture belongs to the Store. It compares canonical persisted
configuration before exposing verified state, while preserving original file
bytes for restoration, including harmless formatting differences. Rename retains
client-coverage and credential-retirement decisions; it does not decode or
compare configuration again. The existing Snapshot and guarded writer own
capture, exact-mode convergence and platform permissions. Callers receive only
the result they consume, not a second snapshot or recovery protocol.

The `c57bf30` runtime-resolution comparison used the same fixture, Go toolchain
and macOS arm64 host with a native Go overlay for the predecessor. It measured
450.2–479.6 ns/op, 2,528 B/op and seven allocations before removal of the clone,
versus 28.15–28.46 ns/op and zero allocations afterward. This is a historical
function-level observation, not a current or cross-platform performance budget.

Release chronology uses the already-locked Masterminds SemVer implementation
shared by artifact admission and publication. Exact identity preserves build
metadata for tag/epoch matching; precedence ignores it. One adjacent-entry
comparison enforces strict descending order and uniqueness. Repository tests
exercise heading/tag binding, valid metadata, invalid numeric prereleases and
core overflow rather than reimplementing the dependency's parser.

The separate active-Change publication check remains temporarily necessary:
it is read-only, but still repeats branch classification locally. Removing it
without a proven hosted ETHOS replacement would erase publication admission.
Task 8.11 retains that ownership gap; SemVer simplification does not close it.

Release sources have one private owner in `internal/upgrade/source.go`:
configuration resolution, validation, peer dispatch, and unavailability
classification. An update resolves environment overrides once; both metadata
and asset transports consume the selected source. GitLab's private transport
uses the bound source without resolving the environment again. Source-policy
tests live with that owner, while GitLab tests retain CLI and HTTP behavior.
The duplicated provider-resolution functions and value-receiver restoration
are removed without adding a client layer or compatibility mode.

Release identity follows the same ownership rule. Construction, readiness and
publication use the already-admitted SemVer parser used by upgrade. Remove the
release-only regular expression, dot-count validation and substring-based
stability checks; preserve each entry point's error context and its separate
signing or source-admission contract. Publication retains the parsed identity
through asset selection and prerelease classification without another wrapper
package or duplicated grammar.

CUE projects mandatory release admission before construction on both Forges;
it delegates version classification to the release command rather than
reimplementing channels in workflow expressions. Release readiness owns
executable admission only. Remove its unused historical-prose blacklist and
command, leaving document quality with the existing native checks.

Embedded release-source validation belongs to the construction request. The
source-only command and actual build share that validator, including complete
Forge coordinates and repository path semantics. This removes the weaker
parallel build check without coupling release construction to runtime override
policy. Neither Forge becomes mandatory and no new package is introduced.

The source-boundary review found that empty hostnames and empty query or
fragment markers passed both origin checks. Runtime additionally admitted a
repository without a namespace, while encoded separators could change routing
meaning. Both existing owners now compare the parsed origin with a native
scheme-and-authority URL and use native relative-path and URL-escaping contracts
for repository coordinates. This removes manual segment loops without adding
a shared framework or coupling runtime overrides to build policy. IPv6, explicit
ports, one root slash and ordinary namespace punctuation retain positive tests.
Runtime private HTTP remains distinct from HTTPS-only embedded metadata.
Public updater tests require zero HTTP requests and helper calls on rejected
coordinates; build-entry tests require zero tool calls. Both validators pass the
cyclomatic-20 trial, with behavior and failure boundaries preserved.

Identity migration separates complete transactions from CLI presentation.
`internal/renaming.Service` owns rename and verified finalization; its callers
do not coordinate credential copies, configuration commits, or cleanup.
`internal/cli/renaming` owns flags, interactive identity selection and rendering,
using the existing invocation context instead of a second root-level dependency
assembly. Domain tests own cancellation, retention and commit failure; CLI tests
own argument routing, interaction and output. The old command API and duplicate
prompt contract are removed rather than retained as forwarding compatibility.

Codex catalogue transformation has one owner in `internal/codex/catalog`:
validated identity, unmodified client metadata and deterministic alias
projection. It preserves the full document, not only model entries, and returns
the uniquely matched base alongside the projection. Product reconciliation and
repository acceptance consume that same contract. `internal/codex` retains
isolated executable observation and transactional projection;
`tools/codex/catalog` owns measurements and acceptance verdicts. Pure document
tests move with the transformation, while repository verification tests move
out of the product package. The duplicate parser and unused probe renderer are
removed, without forwarding exports or another catalogue schema.

Bundled and effective catalogue observations share one private probe lifetime:
an isolated home, bounded child execution, and exact cross-platform cleanup.
Executable identity remains available when catalogue retrieval fails; cleanup
errors retain the original command cause and name the owned home. Repository
verification separately owns its projected input file and reports its cleanup
failures. Existing native-process fixtures replace the two shell-only product
tests, preserving identity, user configuration and lifecycle assertions on every
host. Unix permission-denial cases remain explicitly platform-scoped rather than
being presented as Windows evidence. No public probe framework is introduced.
The two lifecycle tests share one private fixture that owns ambient settings,
temporary-directory selection and permission restoration. Preservation checks
run during teardown even after an assertion fails. Native Go overlays confirm
unchanged detection of skipped cleanup, discarded cleanup errors and ambient
home reuse; the covered product blocks remain identical.

Setup transaction ownership belongs to the existing synchronization package.
`Synchronizer.Setup` admits first-time configuration, validates desired
configuration, Token Account ownership and selected client scope, then commits
credentials and client projections
with owned compensation. CLI onboarding provides input and result presentation;
it no longer carries a credential rollback transaction. Domain tests cover
preflight, cancellation and backend preservation; CLI tests retain guided and
manifest journey behavior. Credential replacement mechanics remain with the
secret storage owner. This removes the CLI transaction rather than introducing
a setup framework or compatibility wrapper.

First-time admission has one owner in synchronization. The command forms use
that same read-only decision before prompting or probing; Setup enforces it
before mutation. Existing Profiles require import, selection or credential
rotation rather than Setup. Existing credential slots remain valid inputs to
first-time configuration. This boundary removes three CLI-owned definitions
without expanding Setup into a second configuration-and-rotation transaction.
The domain regression checks that rejected setup performs no discovery,
credential write, native-client invocation or configuration change. A separate
public manifest-setup regression reproduced a native login writing credentials
before failing, beyond AIGW's compensation boundary. The working change removes
that duplicate persistence and its binding/rollback interfaces rather than
expanding ownership into client-private storage. Public setup now preserves
seeded client-owned credentials and uses command-backed Account Tokens.

Both helpers match a fingerprint of the projected client, Account and endpoint
against the current Route before reading a Token. Account or endpoint changes
require synchronization and client reload; Token rotation, model and label
changes retain the matching credential identity. Focused tests cover these
cases and prove no Token access on mismatch. The fingerprint is neither secret
nor caller authorization. Client-native authentication remains client-owned.

Task 2.7 is verified at the owned-write boundary. Public setup failure after
configuration commit restores the exact pre-existing file map and Token state,
leaves no payload/backup/sidecar residue, and releases the mutation lock for
immediate reacquisition. The inert lock file remains the shared synchronization
identity, not an active lock. Success preserves seeded client-owned credentials.
Affected module/race tests and the macOS isolated portable lifecycle pass.

An isolated Codex 0.153.4 probe consumed the built AIGW helper from a path with
spaces and sent the synthetic Token to a loopback Responses endpoint, preserving
client-owned authentication bytes. The endpoint deliberately returned an error:
this proves credential transport, not inference. Real-client refresh timing,
Desktop and Windows/Linux invocation remain in the native acceptance phase;
no immediate hot-refresh guarantee follows from helper support.

Forge tooling separates command parsing, object provenance and peer transport
within its existing package. Publication and read-only inspection call the same
typed verifier, without parsing a second command line. The verifier binds the
commit policy and tracked identity metadata to the resolved source commit, so
another checkout or an uncommitted policy cannot alter its result. Signed local
Git fixtures prove both inspection and publication against a different checked-
out policy; no real peer or historical product object is modified.

Ancestry inspection consumes the exact observed peer commit. If its object is
already local, no fetch is needed; otherwise Git fetches that object without
writing references or `FETCH_HEAD`. The temporary observation-ref mechanism is
removed. Git's distinct ancestry result and execution failures stay distinct:
transport or object errors cannot masquerade as divergence and invite a forced
cutover. Signed local fixtures exercise both object paths, preserve the complete
reference set and existing fetch observation, and retain exact-lease publication
admission for genuine divergence.

### Team catalogue presentation has one owner

The authoring contract in `docs/guides/team-rollout.md` separates stable Profile
keys, exact provider model IDs, display identity, optional workflow purpose,
and Route recommendations. The team and workstation model catalogues assign
no workflow roles and omit purposes consistently. Their Profile records now
match: seven selected model families with DMXAPI's retained channel variants.
AIHubMix uses its documented backup API domain and includes Fable 5.1 and Astra.
Existing tests enforce the team artifact's display structure and native export
layout, not a second parser or provider-name registry. Model listing, protocol
success, and native-client success remain separate evidence obligations.

Catalogue admission distinguishes provider listing, authenticated streaming
inference, and the installed client's actual invocation. Verification records
the client identity and preserves configuration bytes; one-turn inference does
not establish context capacity, tool replay, platform support or release
lifecycle. Client upgrades retain the existing installation owner, and any
stable-to-latest channel change remains an explicit compatibility decision.

`models` and `catalog` share the same credential and HTTP observation path.
Profile comparisons report only catalogue membership and preserve the reason
an Account could not be observed. Listing is not reachability; absence is not
proof of unavailability. An incomplete or oversized HTTP body cannot establish
membership. This replaces the independent query loop and its helper rather
than adding another readiness state machine.

### Existing ecosystem tools remain the development control plane

Optional semantic indexes bind to the exact Work Lane root and reuse its
existing project identity. Fresh per-path coverage metadata admits graph use;
`ready` and live Git metadata do not establish that the graph was rebuilt.
The native `.cbmignore` includes authored coverage tooling and policy rather
than accepting the indexer's generated-output name heuristic. No index database,
watcher wrapper, or second source authority is added to the repository.

`mise` is the sole cross-platform entrypoint and calls Go modules, npm, and the
existing repository verification owners. Each Work Lane owns mutable
`node_modules`, build, coverage, and temporary state; lanes share only
content-addressed caches. Minimal `bootstrap`, `check`, `native`, and `release`
tasks replace command memorization without adding shell wrappers.

Bootstrap uses native `go mod tidy -diff` before
`npm ci --include=dev --ignore-scripts`. The explicit development dependency
selection preserves the task's purpose under production-oriented caller
settings without modifying those settings.
It fills the Go source and dependency-test graph without rewriting locks, not
merely the subset needed to compile AIGW. Both Forge projections invoke this
same task; there is no separate hosted npm-only environment definition.

The Go environment is declared once in `mise.toml`: ignore persisted user Go
settings and external workspaces, and use the selected bundled compiler rather
than automatic toolchain downloads. A real subprocess test checks conflicting
inputs, module/compiler identity and preservation of foreign files. Deliberate
process-level build flags remain inputs; this is not arbitrary-shell isolation.

Bootstrap testing snapshots current Go source and actual mise/Go/npm inputs to
a private checkout with spaces, refreshes Go and npm locks and invokes the
existing task twice offline, once with `NODE_ENV=production` and once with npm's
`omit=dev` setting.
Each pass verifies unchanged input bytes, direct-package versions, executable
version output and stale-file removal. Markdownlint executes a stdin document;
OpenSpec and Prettier use their version commands. A wrong executable version or
failed process is rejected even when package metadata is unchanged. Before
isolating npm configuration, the test preserves the populated cache path:
shared content is retained, not user policy or another lane's dependency tree.
No new runner, network fallback, platform path or retry mechanism is introduced.

Standalone-tool checks read every declaration in `mise.toml` and execute its
native version command through locked mise. Only command/output grammars belong
to the probes; expected versions remain in the manifest. Every declaration has
a probe and every probe has a declared consumer. The shared native CI tool
closure includes Syft, so both Forge projections can execute the full set.
All thirteen probes pass locally; wrong-version and nonzero-exit injections
exercise the actual admission test rather than package metadata alone.

[Review 978bd594](https://github.com/HengYangDS/aigw-cli/actions/runs/34560513281)
passed quality and native acceptance on macOS, Linux and Windows for source
`fcb1889`, including npm re-resolution and all standalone executable identities.
The additional Go re-resolution and complete bootstrap now pass locally; their
hosted acceptance passed on all three platforms in
[review 2b4bb650](https://github.com/HengYangDS/aigw-cli/actions/runs/34562199257).
Earlier macOS mise refresh retained
input bytes but emitted provenance/network warnings; a later offline probe also
retained bytes without resolving unavailable remote metadata. Neither proves
warning-free native mise re-resolution. These observations stay distinct from
fresh-download and installed-product acceptance.

`dependencies:resolve` is the explicit online metadata-verification path. It
uses native mise platform templates and runs two lock refreshes, with native
Git comparison against committed tool inputs before and after each pass.
The CUE authority projects opt-in execution to both Forges; GitHub injects its
workflow token only into that step, while GitLab consumes an operator-provided
upstream token. Job artifacts retain observed lock bytes for review, including
failed refreshes. This does not add a resolver, credential store, or alternate
dependency policy; native metadata/provenance warnings still require inspection.

[The first authenticated refresh](https://github.com/HengYangDS/aigw-cli/actions/runs/34563891141)
resolved all thirteen tools without warnings on each native host. macOS passed
both byte-preserving rounds. Linux and Windows correctly stopped after the first
round added five verified GitHub-attestation facts each. Their retained lock
artifacts were checked against the Forge archive digests and merged with native
Git; only those ten generated verification fields changed. No version, URL,
checksum or foreign-platform metadata was inferred or edited.

[Exact-tree verification of the completed lock](https://github.com/HengYangDS/aigw-cli/actions/runs/34564711165)
passed both native refresh rounds and complete native acceptance on all three
platforms for source `b4f8343`. Every retained lock archive matches its Forge
digest and contains the source lock bytes. Windows recovered two upstream HTTP
500 responses through mise's native retry and completed all thirteen entries in
both rounds; no provenance verification was deferred. The logs retain those
transient warnings. Resolved retries are not unresolved provenance, and a green
exit code alone would not establish either distinction. Task 9.6 is complete;
final-candidate and installed-product acceptance remain separate obligations.

Npm tool invocations are native `package.json` scripts executed by the locked
`node --run` command. Go orchestrates these named checks and consumes the
OpenSpec JSON result; it does not resolve npm `.cmd` launchers or reproduce
Windows shell quoting. Scripts address checkout-local package entrypoints,
so a missing install fails rather than selecting a global substitute. Tests
execute the actual Node child process, including a checkout path containing
spaces and a validator that emits valid JSON before exiting unsuccessfully.
These host tests do not substitute for native Windows acceptance.

Pixi is not introduced because Go is the product language and the current
ecosystem locks already cover the repository. Nix and Bazel would add a second
environment or build authority without proportional value at this scale. The
decision can be revisited only if native-library solving or repository scale
changes materially.

### Quality is a positive responsibility graph

Native tool inputs share the subprocess's explicit checkout root. Taplo receives
Git-selected files relative to that root: absolute paths through a symlinked
parent can otherwise be filtered out against its canonical include scope.
Conformance checks execute malformed TOML and lockfiles from a separate caller
directory and require diagnostics, including on macOS's aliased temporary path.
This retains the native parser and exact Git inventory without new exclusions.

The current-source secret scan shares the CI entrypoint's Git inventory, not
the worktree's entire filesystem. One private regular-file copy preserves
repository-relative paths for native Gitleaks policy. It includes ignored
tracked files and nonignored new files, omits deleted paths and symlink targets,
and is removed after success or failure. This small input adapter is needed
because directory mode accepts no Git file list; it avoids per-file processes,
duplicated ignore rules, a custom scanner, and history-only evidence that misses
current edits. Ignored verification logs do not become authored source merely
because they reside beneath the checkout.

The scan projection uses a hidden operation directory so native Go package
discovery excludes it, including before its module descriptor has been copied.
A real `go list ./...` regression observes source during scanning and verifies
the projection is removed after both scanner success and failure. Git ignore
rules alone do not isolate copied Go source from module discovery.

The architecture policy assigns every tracked carrier one semantic
responsibility. `tools/ci` invokes the required checks; each native tool
configuration owns its applicable rules and scope. Together these executable
owners must cover format, lint, type, architecture, security, dependencies,
documentation, and tests. A parallel list of check names cannot establish that
coverage and is not retained in the architecture policy. Custom checks remain
only for AIGW-specific invariants. Quantitative limits come from protected risks
and measured distributions, not arbitrary severity. Warnings are fixed at the
owner.

Markdown acceptance has three distinct layers: formatter-owned presentation,
linter-owned structural rules, and source-grounded semantic and rendered review.
Diagrams identify actors, edge meanings and trust boundaries; tables compare
like concerns; examples state their platform and prerequisites. Heading and
list structure express ownership and sequence rather than incidental edit order.
Current documentation is rewritten in place; historical OpenSpec records are
reviewed as history, not silently updated into current guidance.

Native Markdown heading rules now reject decorative punctuation and standalone
emphasis that conceals a section from navigation. Conformance preserves question
headings, emphasized sentences and official OpenSpec labels; a deliberately
misaligned table exercises the existing native alignment rule. The complete
current Markdown scope passes without exclusions or a custom parser.

The superseded GitHub-only release record is removed because its surviving
independence and signed-object rationale already belongs to the independent-peer
decision. The register retains stable sequence identities without an empty
redirect record. The control-plane decision no longer implies a mandatory proxy.
Credential documentation distinguishes metadata reachability, persisted backend
choice, actual operation permission and client-process environment inheritance.
These statements are checked against their existing selection and projection
owners, not inferred from a healthy local backend.

Five changed documents produce ten local previews at widths 1200 and 440 with
the locked Markdown and Mermaid implementations. The rendering checks preserve
heading hierarchy and the credential diagram's accessible title and description;
the page has no horizontal overflow. This is local preview evidence, not hosted
Forge rendering or completion of the full visual review. Screenshots and source
digests remain under the source-bound verification output.

The terminal review distinguishes policy quality, measurement correctness,
execution coverage, and product acceptance. A complete carrier inventory does
not prove all required rules run; a green linter does not prove package cohesion;
native compilation does not prove a released client journey.

Coverage separates package observation from statement measurement. Native Go
owns counters and platform file selection. A missing package profile triggers
inspection of those selected files with the standard Go parser; only the
absence of function bodies establishes declaration-only status. Native
zero-statement counters and proven declaration-only packages remain visible as
not applicable, without invented percentages or package exclusions. Packages
with statements still require executed counters and the unchanged aggregate
floor. This admission check does not implement a second statement analyzer.

At source `3a75f7c5`, Go lint includes tests and has no broad test exemption for
complexity. However, its limits are close to the documented existing maxima:
150 function lines, 90 statements, cyclomatic complexity 42, and cognitive
complexity 64. This is a growth ceiling, not evidence of an optimized quality
standard. Tasks 7.3 and 8.5 are reopened: the CLI acceptance suite still needs a
behavioral ownership review, and the structural limits need a risk-based
decision rather than a fit to existing offenders. Prior focused passes remain
valid for their exact revisions; they do not settle these broader obligations.

Use the existing architecture policy for semantic owners, the native Go linter
configuration for executable Go rules, the coverage policy for supported
coverage claims, and the repository CI command for orchestration. CUE consumes
that orchestration rather than defining a second quality standard. Do not add a
parallel quality registry or force identical Python and Go directory layouts.
Cross-repository consistency means equivalent responsibility boundaries and
acceptance semantics, while native configuration formats remain tool-owned.

An architecture verdict requires every declared scan root to exist as a
directory and every inspected dependency declaration to parse. Allowed child
names remain an admission set, not a requirement to create unused packages.
The architecture checker consumes import syntax only; the Go toolchain remains
the owner of full language validation. A partial inspection is an error, not a
successful empty report.

The same boundary applies to OpenSpec: native validation owns document meaning;
the CI consumer binds its findings report to this checkout and admits only a
nonempty `all` scope with consistent successful totals and no findings. It
rejects parent-project discovery and empty success without recreating the
OpenSpec parser. This proves document validation, not completed implementation.

Repository inventories bind Git to the requested checkout's own `.git` entry,
including a linked-worktree file. Parent-repository discovery cannot establish
another root's file set. Architecture scanning supports a Git-free directory by
walking that exact workspace; repository link checks require an actual Git
checkout. This distinction remains explicit rather than hidden in fallback.

File enumeration and tool execution share that root. `links`, `check-go`, and `check-toml`
resolve it once and bind each subprocess working directory to it; an absolute
file list alone cannot select the target Go module. Native conformance runs
the real linter against a separate module through absolute and relative paths,
including spaces, and proves valid input, format drift, type failures and
source preservation. Streaming and captured commands carry the same directory
and environment semantics without changing the parent's working directory.
The generic runner creates no build tree: coverage and release owners create
only their own outputs. This removes the unrelated acceptance-directory
precondition from every quality command.

TOML syntax and formatting consume that same Git inventory for `*.toml` and
`mise.lock`; Taplo owns parsing and layout, not a second directory whitelist.
Its native include policy admits the supplied paths so extensionless TOML is
not silently filtered. Markdownlint discovers `**/*.md` with the checkout's
root `.gitignore`, keeping only the exact OpenSpec archive exclusion. A new
document beside source, tooling, or hidden configuration is therefore checked
without another policy edit. Native conformance proves these scopes, ignored
generated output, parent-ignore isolation, syntax and format failures, and
read-only execution. It does not establish semantic document review, TOML
schema validation, or completed hosted CI acceptance.

Native schema validation precedes the quality sequence: golangci-lint validates
its own policy and GoReleaser validates its release configuration. This closes
the gap between syntactically valid YAML and configuration the consumer accepts,
without another parser, schema copy, or wrapper. An isolated checkout with spaces
exercises the real validators through the shared quality dispatcher: valid
configuration passes, unknown fields and missing files fail, and input bytes
remain unchanged. Other configuration consumers remain independent review
obligations; these two validators do not imply universal schema coverage.

Markdownlint's successful execution does not certify its configuration: native
probes accepted an unknown top-level option, unknown rule and invalid rule
parameter. The repository therefore validates the existing policy using the
CLI schema and strict built-in-rule schema shipped in its locked npm packages.
This repository uses built-in rules; admitting custom rule code would require a
separate policy decision. Base and override rule objects have the same schema
boundary. Missing, malformed or externally referenced schemas fail locally;
there is no downloaded schema fallback or tracked copy of upstream rule names.

The small `tools/ci/markdown` owner uses the maintained Go JSON Schema library
`github.com/santhosh-tekuri/jsonschema/v6`, with the existing YAML parser. It
reuses the CI runner and introduces no new executable, service, rule parser or
JavaScript toolchain. CUE's direct schema import could not resolve the bundled
cross-file references without a generated package tree; the upstream-suggested
Ajv CLI would add a separately maintained executable and older transitive
dependencies. The Go library owns schema semantics and reference handling;
repository code only binds local inputs and the strict schema. Native module
inspection confirms this dependency is absent from the product command's
dependency graph. Focused tests cover valid policy, unknown fields and rules,
invalid options, overrides, missing inputs, schema identity, external-reference
rejection and preservation of input bytes. This is a tooling dependency, not a
new foundational product boundary requiring a separate Decision Record.

The quality command sequence must resolve inside its CUE-declared toolchain,
not merely on a developer's broader PATH. A native `mise which` regression
resolves every distinct direct command under that exact declaration; the Forge
projection test proves both peers consume the same declaration rather than
copying its tool list into a second test authority. Transitive commands and
complete job execution remain covered by their native acceptance paths.

The review covers these independent concerns:

| Concern                          | Acceptance boundary                                                                                                                                            |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Product semantics and topology   | Account, credential, route, projection, process, and upgrade invariants have one owner; callers do not coordinate another owner's internal recovery            |
| Correctness and maintainability  | Go analysis covers product, tooling, and tests; complexity, nesting, arguments, and size have named measurement semantics and risk-justified bounds            |
| Test design and coverage         | Behavioral tests and fixtures have precise scope and resource ownership; supported coverage includes every canonical package without inventing branch evidence |
| Dependencies and security        | Dependency hygiene, dead code, secret handling, vulnerabilities, licenses, SBOM, signatures, and update trust have current executable checks                   |
| Non-code quality                 | Every active config, document, schema, workflow, manifest, and root carrier has the relevant format, lint, validation, and navigation checks                   |
| Development and product delivery | Locked independent lane environments and native artifacts prove setup, sync, credentials, client invocation, update, rollback, and uninstall                   |
| Gate effectiveness               | Isolated conformance cases prove configured tools see the intended scope and propagate failures through local commands, hooks, and all required CI event paths |

For each numeric limit, name the risk, native measurement, semantic scope,
current distribution, largest legitimate cohesive example, false-positive cost,
and review trigger before selecting a bound. Observed maxima alone cannot
justify it. A test table is not equivalent to nested executable control flow;
test setup, assertions, and teardown remain subject to their own risks. Reject
blanket test exclusions, permanent offender baselines, and helper extraction
that changes a metric without reducing state or responsibility. Native
[`golangci-lint` settings](https://golangci-lint.run/docs/linters/configuration/)
remain the first choice; no custom analyzer is added for a metric already owned
by an admitted tool.

File size is measured by locked SCC rather than a repository-owned counter.
The native code-line budget applies to every authored Go file selected by the
checkout inventory, including tests and non-host platform variants. Complete
measurement is required: omitted, repeated or foreign results fail admission.
Relocation preserves whole declarations, package identities and platform tags;
no forwarding function or count-only parameter object is a valid remediation.
The numeric authority is `.config/checks/go/size.toml`, consumed by the shared
`check-source-size` gate, not duplicated in Forge workflows.

The [quality policy](../../../docs/governance/change-and-release-policy.md#quality-and-platform-evidence)
owns native measurement semantics, blind spots, numeric rationale and review
triggers. Nesting rejects a score of five or more across product, tooling and
tests; a bounded score-four path remains admitted. Cyclomatic complexity now
admits at most 25 across those same scopes. Trial 20 first, then evaluate whether
15 improves semantic clarity and verification enough to justify adoption; no
lower target is planned.
Cognitive complexity admits at most 50, function span at most 120 physical lines,
and native function statements at most 65. Parameter and maintainability limits
remain provisional.
For every metric, preserving legitimate cohesive expression takes precedence
over minimizing the number. A lower threshold is adopted only with demonstrated
net benefit, never through shallow extraction, parameter bags or hidden state.
No tightening may relax another limit, behavior assertion or covered scope.
Splitting a cohesive journey to reduce its score does not
satisfy task 8.5.

The companion trial at `eae7e818` reports 38 cyclomatic-20 findings,
24 cognitive-40 findings, 15 nested-conditional-score-four findings,
four parameter-six findings and three statement-65 findings. Earlier
combined counts were incomplete because native line deduplication hid companion
rules on the same declaration. Disable that presentation filter in the one Go
policy; counts are host-selected observations, not a cross-platform census or
an exemption list. Review semantic responsibilities before adopting lower bounds.

The statement-65 boundary is implemented without additional owners. Package
coverage observation absorbs the measured-package loop from command orchestration
and propagates failed writes for both measured and zero-statement reports. The
existing report test now injects these failures independently of aggregate output
and proves temporary-profile cleanup. Bootstrap declares one isolated environment and checks every executable identity
through one matrix. Route checks preserve an explicit file/directory inventory,
including Claude settings. The existing native repair owner now performs and
verifies user edits after each upgrade; the outer lifecycle retains all upgrade,
rollback and uninstall stages. No additional helper is introduced. Native
65/66 cases cover product and test source.

Projection assertions now use an independent native JSON reader with fresh
maps, rather than repeatedly decoding through product helpers or merging into
the previous observation. Model-drift recovery compares the complete expected
document: fault injection that removes the credential helper passes the former
partial assertion and fails the replacement. Existing rollback, ownership,
foreign-field and credential-secrecy checks remain. Explicit receipt and doctor
case tables replace decisions inferred from scenario names and nested boolean
products. All five reviewed Claude settings tests pass the 20/40 trial without
moving their lifecycle steps into a support package.

Command help uses one grouped renderer, including ordinary ungrouped commands,
and preserves Cobra's ordering rather than sorting its output again. Native
metadata tests cover alphabetical defaults, explicitly disabled sorting, mixed
groups, inherited options and unchanged configuration storage. This deletes a
parallel presentation path; it does not change command dispatch or client state.

Keep physical span 120: the cross-checkout conformance test legitimately keeps
its readable source examples and case table next to execution and assertions.
Moving that data out to pass 110 adds navigation without reducing responsibility.
Maintainability 25 also rejects a branch-free declarative table that every other
native rule admits; retain 20 for the quality policy's recorded reason. Parameters
seven and nesting below five remain enforced pending semantic review. Neither
trial permits exclusions, parameter bags or automatic splitting.

The cyclomatic batch keeps complete setup, sync, check and uninstall journeys
while sharing their real filesystem fixtures and native executable naming.
Exact map/object comparisons replace longer partial field assertions and also
reject unexpected observations. Filesystem inspection and ownership inspection,
credential observation and input, verification admission and final responses,
release inputs and SPDX normalization have separate test owners. Existing
assertions and platform conditions remain. The native conformance fixture for
cognitive complexity uses nested loops so the independent 50/51 boundary stays
below the new cyclomatic limit; a cross-metric failure is not accepted as proof
of the intended gate. No product package, schema, runtime state or tool is added.

The next tightening keeps the same journeys while sharing actual finalizer
credential setup and existing Codex sidecar fixtures. Exact Profile and settings
comparisons replace partial checks; dry-run byte equality subsumes duplicate
post-load assertions. Native host invocation has one argument/preflight owner,
publication-source parsing owns allowed branch names, and coverage parsing owns
nonempty package observations. Boundary conformance is 25/26. Lower trials must
preserve these contracts and reduce real complexity before replacing the gate.
Explicit scenario rows remove conditional test-oracle logic without losing
Cartesian cases; independent schema fixtures remove inter-case state. Existing
snapshots preserve exact file identity. The independent cognitive and physical
span boundaries are 50/51 and 120/121; actual gate-loss and ownership-defect
mutations must still be detected. No new metric framework or test exemption is
introduced. Statement conformance is 65/66. Coverage package observation now
belongs to the measured result and preserves all missing-package and inspection
causes; the command retains execution and profile lifetime. Native lifecycle
acceptance reuses its complete upgrade/sync/identity-check operation at both
upgrade points, preserving the full rollback and uninstall journey.

Nesting conformance is 4/5, including host-selected product and test sources.
Existing Codex state recovery, prompt selection and rename rendering owners
lose enclosing branches and duplicated output without new helpers or changed
admission. Full behavioral suites and a disabled-gate mutation establish that
the tighter rule preserves behavior and remains effective.

Diagram grammar is checked by the existing CI command plane using the locked
Mermaid validation library, not a custom parser or browser-dependent CI framework.
The shared checkout inventory includes current and historical tracked diagrams,
and local untracked authored documents. Conformance rejects the reproduced
sequence-message semicolon error and missing local supply while preserving
source bytes. The exact file inventory crosses standard input rather than the
command line; no ambient parent configuration can change the validation.
Native browser rendering remains separate visual evidence. The transaction
diagram uses a decision flow instead of nested sequence frames: its purpose is
to explain guarded writes and compensation, not concurrent participant timing.
Short centered labels remain inside nodes rather than crossing lifelines.

Configuration admission keeps collection and reference ordering in Config,
Account constraints in Account, and Profile authentication in Profile. Stable
key, query-parameter and endpoint traversal produces one reproducible diagnostic
without mutation. Historical native measurements fell from cognitive 62 to 19
and cyclomatic 37 to 14 for Config, with Account/Profile cognitive scores of
13/16; these observations do not justify a repository-wide threshold.

The CI dispatcher owns argument admission; Go, Markdown, TOML and secret checks
own complete input selection and execution. Go checks deduplicate sorted
packages after formatting and preserve either phase's original error. Source
and quality share dispatch rather than duplicating it. This removed mutable
cross-check command state and redundant path conversion; the dispatcher's
historical cognitive score fell from 51 to 31.

The existing cross-checkout conformance test exercises formatting, typing,
nesting and five exact structural boundary pairs in applicable product/test
source. Standard Go formatting prepares fixtures; actual native checks prove
admission, rejection and unchanged source bytes. Disabling their linters through
an isolated overlay makes all five over-limit expectations fail. Tasks 8.5 and
8.10 still require justified remaining limits, analyzer-gap decisions and
complete hook/hosted wiring; this evidence closes none of those by implication.

Forge object verification matches the complete commit subject against the
selected revision's policy, consistent with ETHOS hook matching rather than
accepting a matching substring. Signed-object regressions exercise unanchored
alternation, prefixes and suffixes. Individual-tag and tag-set verification use
the existing strict SemVer parser; the handwritten tag regex and duplicate
set-level validation are removed. Tests accept build metadata and reject
numeric prerelease leading zeroes without changing signature verification.

The quality work executes in a fixed order within existing tasks:

1. Reconcile effective policy, scope, measurements, native tool settings, local
   commands, hooks, CUE event routes, and completion claims against current code.
2. Define justified final bounds and conformance tests before changing the
   gate. Fix omitted or false-success paths without creating another checker.
3. Simplify one complete product invariant and its behavioral tests at a time,
   starting with credential/projection safety and upgrade recovery; then close
   tooling, documentation, configuration, and naming gaps. Delete displaced code.
4. Activate the final rules without permanent exemptions. Batch mechanical
   formatting; use focused tests before the affected gate, not full CI to
   discover syntax or policy errors.
5. Freeze the complete candidate for full quality, native artifact, client,
   security, performance, release and peer acceptance. Only then archive and
   retire the authoring lane and unused outputs.

ETHOS owns generic hook dispatch, scope admission and lifecycle defects. AIGW
owns its product policy, rule applicability, tests and CI, and does not wait for
ETHOS to repair those local responsibilities.

### CUE owns CI semantics; Forges own syntax and capacity

One CUE graph describes facts and dependencies. GitHub Actions and GitLab CI are
generated projections. Jobs are separated by independently useful evidence:
fast quality, Go compatibility, native product journeys, release construction,
and publication. Platform proof follows real runner capability; superficial job
symmetry is not required, semantic parity is.

Branch role values are consumed from `.ethos/workspace.toml` by CUE's native
TOML reader, not duplicated under CI-specific meanings. The projection command
passes both authoritative inputs; missing workspace data fails rendering before
any output is written. The [event routing decision](../../../docs/decisions/dr-0010-lifecycle-scoped-ci-evidence.md)
records why accepted `dev` and release `main` both execute verification until
cross-event reuse can verify equivalent evidence. Current projections retain
their exact bytes while the declaration loses its contradictory role names.
Generic local retirement follows current ETHOS decisions rather than copied
main/peer/install prerequisites in AIGW documentation.

### Evidence is attached to claims, not accumulated as a second history

GitLab tool caching retains installed tools together with mise's native cache
metadata, including incomplete-install markers. Its native `when: always`
policy preserves completed installations after a later failure without masking
that failure. The key includes both directory roles; older installs-only
archives cannot silently omit completion metadata. A network-disabled Linux
probe executes a restored locked Node installation, then confirms that adding
its native incomplete marker makes the identical bytes unavailable to mise.
This removes repeated completed downloads, not the runner's slow upstream link.

Program replacement and client projection are distinct transactions. Upgrade
directs the operator to the active executable's `sync`. Rollback first withdraws
enabled integrations with the current executable, then restores the predecessor
and recreates its projections; an older synchronizer may otherwise skip a newer
projection when domain values are unchanged. Native lifecycle acceptance runs
that sequence and executes the projected helper through the platform shell,
not a reconstructed current-version argument list. Historical-schema migration
is a separate external-data obligation; a synthetic predecessor cannot prove it.

Native release acceptance resolves GoReleaser inventory paths against the same
repository root used for construction before checking containment in the owned
stage. Both absolute and build-root-relative inventory paths are valid; paths
outside the stage remain invalid. The fixture exercises both forms so placing
temporary output inside the repository does not invalidate legitimate products.

Real-client lifecycle acceptance belongs to the existing release test owner,
not a second platform-specific script suite. An explicit Go build tag admits
the client journey; required absolute artifact and client inputs fail closed.
The existing fixtures own installation, replacement, withdrawal and teardown.
A standard-library HTTP server supplies authenticated Responses and Messages
streams, with native route/method matching and independent envelope tests.
The source quality command always lints this source and runs its offline
contracts. Real-client execution remains a separate native acceptance claim.
Fixture initialization removes inherited credentials; subsequent changes
replace only their named variable and preserve deliberately supplied test
credentials. No product runtime, credential backend or service is changed.

Local gates, native jobs, release assets, and installed-runtime observations
are referenced by exact commit and immutable digest. Generated reports remain
ephemeral unless a durable consumer requires them. OpenSpec records intent and
progress; Git and release objects retain history; empty evidence shells,
records directories, stale lanes, proposals, tags, runtimes, and temporary
services are deleted when they have no consumer.

### ETHOS governs repository transitions, not product behavior

AIGW uses current ETHOS status, prewrite, proof, archive, land, and retirement
commands. Missing or defective generic lifecycle behavior is reported to ETHOS
and does not become a copied AIGW state machine. Independent product work
continues when the affected transition is not on its critical path.

## Risks / Trade-offs

- **Breaking removal exposes hidden users** → search imports, configuration,
  docs, released fixtures, and current host state before deletion; provide one
  data migration only when a supported state still has a consumer.
- **Repository-wide restructuring creates noisy diffs** → move one semantic
  owner at a time, keep behavior tests green, and avoid simultaneous cosmetic
  churn.
- **Strict tools overwhelm product work** → introduce each tool with complete
  scope and zero unexplained baseline, then remove the custom checker it
  supersedes.
- **Native evidence is slow or unavailable** → keep fast portable contracts
  local, schedule independent native jobs, and report capability absence rather
  than leaving a pipeline pending or weakening the claim.
- **Credential probing triggers host UI** → separate value-free observation
  from authenticated reads and exercise the exact native API on each platform.
- **Dependency freshness breaks reproducibility** → update one locked supply
  chain closure, regenerate deterministically, and validate clean-room and
  released artifacts before replacing the installed baseline.

## Migration Plan

1. Freeze current repository, Forge, installed-program, configuration, client,
   credential-backend, and external-endpoint facts without mutating them.
2. Close the setup, credential, selection, synchronization, and readiness
   journey with failing acceptance contracts and one state vocabulary.
3. Enforce the AIGW and optional external-gateway boundary, then prove direct
   and loopback Accounts through the same public interface.
4. Converge credential backends and client Adapters, including rollback and
   uninstall of only AIGW-owned state.
5. Derive and migrate semantic packages, tests, tools, configuration, and
   documentation; delete replaced entities in the same closure.
6. Install the positive quality responsibility graph and remove redundant
   custom mechanics.
7. Add the minimal locked development tasks, advance the supply chain, and
   prove deterministic clean-room reconstruction.
8. Generate both CI projections from CUE and obtain exact-commit native evidence
   on macOS, Linux, and Windows.
9. Build a signed release candidate; verify fresh install, update from the
   retained baseline, rollback, uninstall, credential modes, and real Codex and
   Claude projection journeys.
10. Archive and land only after all tasks are proved; synchronize local and both
    Forge `main` and `dev`, publish the signed tag and identical assets, then
    remove the proposal, Work Lane, obsolete runtimes, and other owned residue.
