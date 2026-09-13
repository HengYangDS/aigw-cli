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
An active Proxy P0 incident may interrupt this order only for bounded repair,
release and runtime verification; unrelated Proxy refactoring remains deferred.
This sequencing does not reduce either product's terminal requirements.

The remaining execution order is dependency-driven. Task checkboxes remain the
only progress ledger; this table defines closure boundaries, not another status
store. A contradiction reopens its owning task instead of adding a parallel
mechanism or preserving an unsupported completion claim.

| Order | Existing tasks            | Closure and acceptance                                                                                                                                                                                                     |
| ----- | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | 7.2, 7.3, 7.5, 7.8        | Consolidate semantic owners and behavioral evidence; delete duplicate fixtures, suffix-only families and forwarding layers. Preserve native platform selection and prove complete recovery at the owning boundary.         |
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

Bulk client verification uses the current configuration's enabled Adapter set,
not the product's supported-client registry. Configuration owns that stable
selection. The verification command preflights every enabled Route before
invoking any client and saves only the successfully verified scope. An empty
enabled set returns an explicit no-client result rather than an empty proof.

Account retirement consumes the same set. A current checkpoint must cover every
enabled client; with none enabled, credential equality and guarded backup
convergence suffice. Missing source credentials need no replacement Token.
Current-configuration equality, credential rotation consent, exact preimages and
retryable cleanup remain unchanged. This removes the all-supported-client
validator rather than adding a scope registry or another checkpoint format.

Implementation order for this correction is behavioral RED at the public CLI,
the existing configuration/finalization owners, focused GREEN and race tests,
then source quality and packaged single-client acceptance. Existing task 11.4
owns the correction; it creates neither a parallel Change nor a release claim.

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
A package earns its boundary through one responsibility, dependency direction,
change reason and recovery lifetime. Moving files, shortening functions or
adding a forwarding interface does not establish an abstraction. Prefer an
existing domain owner or the standard library before introducing another one.

#### Repository-wide ownership review

The review covers every tracked carrier, not only Go files. The current
inventory contains 1,020 files: 438 current carriers and 582 official OpenSpec
archive files. Native Go selection resolves 59 packages for each of the six
supported OS/architecture combinations. These are inventory observations, not
new hard-coded policy limits.

| Surface                      | Semantic owner and placement                                                                                                                        | Retention and acceptance boundary                                                                                                            |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Entrypoint and commands      | `cmd/aigw` starts the program; `internal/cli` assembles operation-owned packages.                                                                   | Cobra owns argument admission, help and completion. Command leaves do not import peers.                                                      |
| Configuration                | `internal/configuration` owns Accounts, Profiles, Routes, Adapter declarations, manifests and persistence.                                          | Validation, cloning, checkpoints and backup recovery share this owner; a manifest is not a second runtime model.                             |
| Workflow                     | `internal/synchronization` owns setup, scoped selection and commit; `internal/renaming` owns identity migration.                                    | Callers supply intent rather than coordinate credential writes, rollback or verified retirement.                                             |
| Clients                      | `internal/client` composes adapters; `internal/codex` and `internal/claude` own native projections.                                                 | Pure Codex catalogue transformation has its own subpackage; executable observation and reconciliation stay with the adapter.                 |
| Credentials and observation  | `secrets` owns storage; `credential` owns authentication requests; `diagnostics` and `readiness` own observation and state vocabulary.              | Storage, authentication, catalogue membership and inference are different claims. Provider diagnostics stay below `providers`.               |
| Host capabilities            | `discovery`, `surface`, `platform`, `process`, `transaction`, `console`, `prompt`, `presentation` and `redaction` own narrow capabilities.          | Each has current callers and a distinct change reason; no catch-all package combines filesystem, UI and process authority.                   |
| Product update               | `internal/upgrade` owns source selection, peer transport and replacement; `artifact` owns archive admission.                                        | Artifact parsing does not depend on the updater. Public acceptance has an independent fixture lifetime.                                      |
| Repository quality           | `tools/ci` executes native tools; `projection` and `markdown` own distinct inputs. `architecture`, `coverage` and `repository` own declared checks. | Configuration contains data, never implementation. Generic protected-lifecycle overlap remains an explicit open governance obligation.       |
| Release                      | `tools/release` composes `construction`, `artifact`, `readiness` and `publication`; `tools/forge` owns signed-object transport and provenance.      | Native journeys belong to release, not CI dispatch. Transport and orchestration retain separate carriers within one package.                 |
| Root and configuration       | Native discovery, locks, identity, licensing and reader entrypoints stay at root; eleven `.config` carriers own explicit policies.                  | The [authority map](../../../docs/governance/change-and-release-policy.md#authority-map) identifies consumers without a duplicate inventory. |
| Documents and manifest       | The 22 documents follow audience and responsibility; `docs/README.md` is the sole directory index. `manifests/team.toml` owns team capability.      | Research is not an adopted decision. Physical placement does not certify rendering, live models or team setup.                               |
| Specification and governance | Official OpenSpec owns specifications, one active Change and archived intent. Three `.ethos` files declare adoption, workspace and publication.     | Transient compilation, proof and coordination remain ETHOS-owned in the Git common directory; no second lifecycle state is added.            |
| Generated and local output   | CUE owns both Forge projections. Locks are retained inputs; build, verification and developer-tool output stays ignored.                            | Follow [output ownership](../../../CONTRIBUTING.md#output-ownership-and-cleanup). Caches and receipts never acquire product authority.       |

All six native import inventories have no product-to-tool edge. Test-only
references cannot justify dormant production allowances. A graph resolver's
same-name call match is a navigation hint, not proof of a Go import; the native
compiler and explicit imports decide that boundary.

Flat filenames are not inherently separate modules. Retain private access where
one transaction needs it and native OS suffixes where the compiler selects
behavior. Split when ownership, visibility or fixture lifetime changes, not
merely to meet a line budget. An unrelated concern cannot remain in a file
merely because it fits that budget.

#### Behavioral and test ownership

| Responsibility                      | Test placement                                          | Reason                                                                                                          |
| ----------------------------------- | ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Public CLI journeys                 | `internal/cli/acceptance`                               | One isolated application fixture, grouped by user operation; manifest setup remains distinct.                   |
| Command admission and presentation  | Relevant command package and CLI root                   | Flags, errors, help and completion do not need credentials or network fixtures.                                 |
| Credential durability               | `internal/secrets`                                      | Typed slots, observation, file identity, replacement and native vault behavior need private seams.              |
| Client composition                  | `internal/client`                                       | Admission order, compensation and future-adapter conformance are distinct contracts.                            |
| Client projection                   | `internal/claude`, `internal/codex` and Codex `catalog` | Public external-package tests remain external; private recovery and pure transformation stay with their owners. |
| Program update                      | `internal/upgrade`, `artifact` and `acceptance`         | Transport, archive validation and process-isolated journeys have different lifetimes and visibility.            |
| Packaged lifecycle and real clients | `tools/release`                                         | Tests consume built artifacts and measured client identities; tool fixtures do not become product dependencies. |

Shared fixtures hide genuinely shared setup, not alternative behaviors behind
boolean modes. Scenario-only fixtures stay beside their consumers. The real
in-memory Store supports injected tests; fault wrappers add only the error
being exercised. It is not an operator-selectable persistent backend. Do not
add an exported support framework or duplicate Store/Runner merely to move tests.

OS suffixes and build constraints are executable selection. Keep Unix-only
credential and permission cases scoped; retain real Windows handle/namespace
checks rather than treating compilation as execution. Relocation preserves
declarations, assertions and platform selection, not just test counts. Assert
outputs and owned effects rather than enumerating retired helper names.

#### One owner per transaction

- **Configuration:** cloning isolates nested Account diagnostics and Adapter
  targets. Runtime resolution is read-only and does not clone or initialize the
  model. One validator admits complete checkpoint JSON and unique admitted
  client scopes. Backup capture compares canonical configuration while retaining
  original bytes and permissions for restoration.
- **Setup and selection:** synchronization admits first-time setup before
  prompting or probing, validates Token ownership and scope, then owns credential
  and projection compensation. Selection derives its client from the Profile.
  Scoped edits preserve unrelated client preferences and conflicts; explicit
  sync/repair remains intentional reconciliation.
- **Client composition:** admission order controls discovery, planning, apply
  and reverse compensation, regardless of constructor order. Adapter receipts
  stay internal; callers receive no unused aggregate rollback obligation.
  Compensation attempts every independent target and joins original errors.
- **Cancellation:** check before preparation and each new effect. Between
  adapters, compensate completed effects; do not report a completed final commit
  as canceled. Once replacement starts renaming, complete or compensate it.
- **File durability:** the existing snapshot and guarded writer own bytes,
  digest, existence, permissions and equality. Absent and empty files differ.
  Close staging handles once, retain write and close failures, and remove only
  exact owned staging names. A committed rename relinquishes its former name.
- **Recovery:** Claude, Codex and configuration restore owned files independently,
  preserve later user edits and join conflicts. Restored checkpoint bytes
  describe captured configuration, not new external verification. Credential
  storage retains stronger constant-time and filesystem-identity checks.
- **Credentials:** one typed view exposes Store operations over purpose-aware
  backends. Validate Account identity before resolution; repeated purpose
  selection is idempotent. Preserve identity, read-only behavior, inspection and
  compensation without default-Token forwarding on every backend.
- **Credential helpers:** match projected client, Account and endpoint to the
  current Route before reading a Token. Model, label and Token rotation preserve
  that identity. Mismatch yields no Token; native authentication and conversation
  data remain outside AIGW ownership.

Credential retirement owns one complete delete-and-confirm operation for both
Token and diagnostic slots. Confirmation uses metadata-only presence, never
credential-value reads or JSON decoding. Deletion and observation failures keep
their original causes, and independent slots still attempt cleanup. Route Token
acquisition is a private, non-mutating decision separate from synchronization;
existing and client-native credentials return before prompting. GitHub release
verification returns before creation, so publication no longer carries a mutable
created flag through nested branches. These semantic boundaries permit the
stricter nesting rule without statement tricks, extra packages or exclusions.

#### Transport and artifact boundaries

Authentication and diagnostics reuse one credential-domain HTTP boundary;
consumers interpret results according to their own claims. Native redirect
limits, origin/TLS protection, bounded complete-body consumption and read/close
errors remain effective. Provider pagination exhaustion is incomplete observation,
not evidence of Token absence.

Catalogue presentation consumes the configuration domain's ordered Profile IDs.
Account traversal uses native `maps.Keys` and `slices.Sorted`, not another sorting
helper. Tests assert ordered rows and discovered Accounts; independent native
Go overlays reverse each order and must fail. Similar-looking backup/checkpoint
readers and Account/Profile edits retain different schemas, intent and errors
rather than being forced into a generic framework.

Codex target preparation owns snapshot capture and desired projection in one
operation. The single-caller seven-argument preparation layer is removed; one
convergence observation determines both transaction-ID retention and the public
plan classification. A converged target prepares no writes and preserves every
owned file. The artifact dependency-ordering owner remains distinct because
creation and withdrawal require opposite catalogue ordering.

The pure Codex catalogue owner preserves the complete native document and
returns the uniquely matched base. Product reconciliation and repository
acceptance share this contract. Executable identity remains observable after
catalogue retrieval failure. Each real-client probe owns a private home and
cleanup; cleanup failure retains the invocation cause and exact path.

Upgrade resolves source overrides once for metadata and asset transports.
Unavailability differs from authorization/integrity failure. Preserve origin,
redirect-chain credential and TLS protection. Archive admission returns the
measured digest rather than rehashing for peer comparison. Runtime private HTTP
and HTTPS-only embedded metadata remain different trust policies.

Update and rollback share recoverable executable replacement: preserve the
predecessor, stage durably, retry only the narrow native rename, and compensate
activation failure. Preserve UNC and extended Windows prefixes. Temporary and
persistent handle-lock tests are native evidence; archive shape and synthetic
version runners are not.

Construction owns its workspace and unique sibling backup. Failed publication
restores previous output; failed restoration retains the exact backup and both
causes. Successful publication followed by cleanup failure must say publication
occurred. Two renames are recoverable replacement, not uninterrupted atomic
directory visibility.

The locked SemVer implementation owns grammar and precedence. Exact identity
retains build metadata for tag/epoch matching. Construction, readiness, update
and publication keep their error context without parallel regexes or a new
shared-version package. Source validation rejects ambiguous origins and encoded
separators before network/helper calls; runtime and build policies retain their
intentional trust difference.

Forge verification reads policy from the exact source object, not another
checkout or uncommitted file. Already-local peer objects need no fetch; otherwise
fetch the exact object without observation refs or `FETCH_HEAD` mutation.
Execution failure is not divergence. Publish locally signed objects unchanged
with explicit expected-state protection. Every branch update carries Git's exact
observed-tip lease, including creation; an explicitly supplied expected tip must
match even when the update is a fast-forward or already equal. Ancestry grants
fast-forward eligibility, not permission to ignore the caller's expected state.
Real signed-repository tests verify rejection before either ref changes and
inspect Git's native trace for both atomic-update leases. No alternate push
implementation or permanent hook bypass is part of this behavior.

#### Migration boundaries

The catalogue cleanup removes two functions and adds no module, file, dependency
or policy. Native duplicate/unused analysis and six-target imports inform review;
green tools alone do not certify semantic quality. Source, tests, specifications
and focused mutation checks remain the behavioral evidence.

The physical-topology review closes task 7.2, not these separate obligations:

- **7.5 and 8.11:** finish global consumer/dependency review and resolve generic
  protected-lifecycle overlap. Retain the existing read-only admission until an
  equivalent detached-checkout gate is demonstrated; do not add a lifecycle
  engine or block unrelated product work.
- **8.1, 8.5 and 8.10:** prove effective concern coverage, defensible quantitative
  limits and local/hosted failure paths. A package map is not gate proof.
- **11.4–11.8:** verify CLI semantics, team-model claims, rendering, links and
  user journeys. Correct placement is not content acceptance.
- **12 and 13:** complete exact published-artifact/platform evidence, final
  consumer/residue audits, release and lane retirement. Historical proof does
  not replace current observations.

Git and existing source-bound output retain implementation history; this section
owns surviving decisions, not a growing repair diary.

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

Native early mise configuration owns discovery for local development and both
Forges. It excludes parent, user and system policy using native path templates;
CUE does not duplicate that boundary with environment overrides or a fictional
empty configuration file. Real subprocess conformance runs each environment
from the checkout root and nested directories while preserving foreign files.
Shared caches and deliberate process-level overrides remain separate inputs.

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

Portable installation inspection belongs to the existing upgrade owner.
`aigw installation --json` exposes a schema-versioned observation of the current
command, resolved program bytes and optional retained predecessor. It neither
executes a retained binary nor reads Account configuration. The output is
computed, not persisted in another manifest or workstation registry. One
rollback-path rule serves install, update, rollback, uninstall and inspection.
The result describes bytes and paths, not release trust or runtime readiness;
those retain their own verification requirements.

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
