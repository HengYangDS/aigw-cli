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

Use the [authority map](../../../docs/governance/change-and-release-policy.md#authority-map)
for source ownership and the [package architecture](../../../docs/architecture/authority-and-projection-boundary.md#semantic-packages)
for dependency direction; repeating those mappings here creates a second
maintenance surface. An owner changes with its product invariant or measured
repository risk. It retires when no supported state, caller, test or acceptance
obligation consumes it. Generated outputs change only through their source owner.

Client projections retain guarded writes and compensation, not atomic visibility
across independent files. Team configuration remains the credential-free
`manifests/team.toml`; governance declarations remain under their existing
ETHOS and Git owners. A removed interface retires its guidance and generated
projection together, while still-consumed user state follows explicit migration.

This resolution also covers generated host projections: Codex and Claude files
are transactional outputs of their Adapters, while build products and Forge
files are outputs of the release and CI sources above. Host caches, Tokens,
installed binaries, and Forge observations are evidence or state, never a
tracked source of truth.

### Reconciliation findings

Reconciliation belongs to the existing task that owns the violated contract.
Completed baseline findings remain in Git history, not a second stale ledger.

| Disagreement                                                                                          | Owning closure                                |
| ----------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| Canonical specifications retain superseded default-route semantics pending official delta integration | 13.6 OpenSpec closeout                        |
| Source/native checks pass, but final published-byte execution and installation remain unproved        | 12.2, 12.6, 12.8 and 13.6 delivery acceptance |

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

| Order | Existing tasks                    | Closure and acceptance                                                                                                                                                                                                                                                                                                              |
| ----- | --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | 5.4, 12.8, 12.6, 13.6             | Close retained-state upgrade and native-client regressions before matrix construction. Run the verified published predecessor through the unchanged product lifecycle and the separate real-client journey on each supported OS. Reopen contradicted tasks; keep run status and blockers only in tasks.md. Preserve operator state. |
| 2     | 7.5, 11.5, 11.7, 13.1             | Close remaining semantic duplication, cross-format organization and stale claims; hook and event conformance is complete under 8.8 and 8.10. Remove superseded owners during each repair; replace temporary governance escape only after an installed equivalent is proved.                                                         |
| 3     | 11.6, 11.8, 12.6, 12.7, 12.9      | Complete fresh-user and team journeys, real-provider and explicit external-endpoint acceptance, hosted rendering, supported client limits and comparative performance budgets. Synthetic streams, preserved settings and source tests cannot establish these claims.                                                                |
| 4     | 9.7                               | Establish the one signed dependency-update owner and demonstrate lock/projection refresh, checks and object-preserving integration. Recheck stable supply inputs within 9.4–9.6 without creating competing peer proposals or blocking independent product repairs.                                                                  |
| 5     | 13.3–13.5                         | Audit all feedback against its existing task and evidence, freeze a clean signed candidate, and run the complete final quality/native/release obligations. Reopen contradicted tasks; completion counts are not readiness percentages.                                                                                              |
| 6     | 10.7–10.9, 12.2, 12.6, 12.8, 13.6 | Integrate source with its active Change, align local and dual-peer main/dev, and publish one signed release and identical assets. Verify published bytes and install safely; update the same task carrier from observed outcomes before archive. Never predeclare future delivery to satisfy an archive gate.                       |
| 7     | 13.1, 13.7                        | Retire the merged proposal and owned lane; delete obsolete refs, releases, scratch and runtimes only after consumer checks. Preserve the current installation and still-owned rollback material.                                                                                                                                    |

Documentation, naming and configuration changes accompany their owning repair;
they do not wait for a later phase. Removal occurs in each closure, with order 7 as a
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

`setup --from` imports capability and may connect a chosen subset. Imported
`recommended_routes` remain recommendation data in the existing configuration;
`routes` holds actual selections. They are distinct meanings, not alternative
authorities for the same selection. Setup and sync share the configuration
selector: preserve every existing selection; for an unselected client, prefer a
usable recommendation, then the same model on a usable Account, then stable
Profile identifier order. No provisional-selection flag or history is needed.
Metadata-only credential observation and client-native authentication determine
eligibility, not a provider-name branch. Rename and deletion update recommendation
references in the same transaction, and export lets local selections take
precedence over imported recommendations in the outgoing team manifest.

`use` changes one client Route. `sync` observes only AIGW-owned Tokens plus
installed clients, then converges eligible Routes and projections. `status` describes
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

- **CLI and terminal interaction**
  - **Retained implementation:** Cobra, pflag, Huh, Lip Gloss
  - **Replacement question:** Does another framework remove parsing or presentation work while preserving command grammar and accessibility?
- **Configuration**
  - **Retained implementation:** `go-toml/v2` and the Account/Profile/Route domain
  - **Replacement question:** Can Viper, Koanf, or schema tooling reduce validation and persistence work without a second configuration authority?
- **Tokens**
  - **Retained implementation:** `go-keyring` and the selected portable backend
  - **Replacement question:** Can a backend library preserve non-interactive selection, Unix file invariants, and Windows DPAPI with less owned code?
- **Program update**
  - **Retained implementation:** Shared durable staging and bounded `robustio` operations
  - **Replacement question:** Can an installer or package manager simplify replacement, verification, rollback, recovery, and exact cleanup together?
- **Release**
  - **Retained implementation:** GoReleaser, Syft, OSV-Scanner, OpenSSH, and release tools
  - **Replacement question:** Can native pipes or another release tool replace final-matrix evidence or object-preserving peer publication?
- **Provider diagnostics**
  - **Retained implementation:** Standard HTTP and protocol-specific leaves
  - **Replacement question:** Does an SDK remove actual request or credential complexity? An HTTP server framework does not match this control-plane product.
- **Tests**
  - **Retained implementation:** Go testing, scoped fixtures, and static analyzers
  - **Replacement question:** Can a test library delete repeated setup or improve assertions without adding another test runner?

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

The review covers every tracked carrier, not only Go files. Git defines the
file inventory; native Go selection defines packages for each supported
OS/architecture combination. Source-bound observations belong to task evidence,
not duplicated counts in the design. Official OpenSpec archives retain their
historical role and are not current product instructions.

The [architecture](../../../docs/architecture/authority-and-projection-boundary.md)
and [machine policy](../../../.config/checks/architecture/policy.toml) own the
package map. Review these semantic lifetimes rather than rebuilding that map:

- **Product state:** configuration owns schema, validation, persistence and
  checkpoints; synchronization and renaming own complete workflows and recovery.
- **Native clients:** composition admits Adapters; each client owns its native
  projection. Pure catalogue transformation has its own client subpackage.
- **Credentials and observation:** storage, authentication, optional diagnostics,
  readiness and inference prove different claims and retain different owners.
- **Host and update capabilities:** discovery, filesystem identity, processes,
  presentation and replacement keep narrow contracts. Archive admission stays
  below the updater; product code never imports repository tools.
- **Repository execution:** CI invokes existing check and release owners.
  CUE generates Forge projections, `.config` holds policy data, and native
  discovery and lock files retain their root locations.
- **Documents and intent:** the documentation index owns navigation; official
  OpenSpec owns specifications and Change progress. Research is not a decision,
  and a correctly placed document is not proof of its claims or rendering.
- **Generated state:** verification output and developer indexes stay local.
  ETHOS owns transient compilation, proof and coordination in the Git common
  directory; no second lifecycle state is added.

Every retained package needs a current caller and a distinct reason to change.
The detached-source lifecycle gate is a disclosed governance overlap with a
real CI consumer; replacement requires an equivalent working ETHOS capability.

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

- **Public CLI journeys** live under `internal/cli/acceptance`, using one
  isolated application fixture and operation-specific scenarios. Command
  admission and rendering tests remain with their constructors.
- **Credential durability** tests stay with `internal/secrets`, where private
  seams expose slots, replacement, file identity and native vault behavior.
- **Client composition** tests keep admission order, compensation and Adapter
  conformance together. Native client tests retain their private recovery
  access; public tests remain external packages.
- **Program update** separates transport/replacement tests, archive admission
  and process-isolated acceptance by their different contracts and lifetimes.
- **Packaged and real-client journeys** belong to `tools/release`; they consume
  built artifacts and measured clients. Their fixtures are not product APIs.

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
measured digest rather than rehashing for peer comparison. The existing
`ReleaseSource` value owns address validation for both embedded metadata and
runtime selection. Public hosts require HTTPS; explicit private-network,
link-local, loopback and reserved test authorities retain the existing HTTP
admission. This address policy does not grant credential access, encrypt HTTP,
or replace artifact-signature verification. Token fallback retains its separate
explicit-HTTPS requirement.

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

Carrier placement, consumer review, rule effectiveness and product delivery
are distinct acceptance claims. Their current progress belongs to tasks.md:

- **7.2 and 7.5:** semantic placement and current dependency/consumer review.
- **8:** effective checks, justified thresholds and local/hosted conformance.
- **11:** precise content, rendered documents and actual user journeys.
- **12 and 13:** final artifact, installation, publication and residue evidence.

The branch-based protected-lifecycle check rejected valid source solely because
its official Change remained active. Its consumer did not justify that rule: it
created a dependency cycle when publication and installation were tasks of the
Change. Remove the checker and its dispatch rather than recreate the same policy
in ETHOS or another adapter. The existing source graph retains native OpenSpec
validation, quality, exact-source evidence and signing. Source can be integrated
with its active task carrier; real delivery outcomes update that same carrier
before final archive.

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

Quality has distinct owners, not a second control plane. The
[authority map](../../../docs/governance/change-and-release-policy.md#authority-map)
assigns policy, execution and acceptance; the
[quality contract](../../../docs/governance/change-and-release-policy.md#quality-and-platform-evidence)
defines scope and evidence. Native configuration owns executable rules. This
Change owns design choices and tasks own progress; neither repeats current
thresholds, trial counts or completed-repair history.

#### Checkout-bound execution

The requested checkout owns input selection and subprocess execution. Git's
explicit repository and worktree select tracked files, including tracked-ignored
files, and nonignored authored files. Deleted files are absent. Parent discovery,
an alternate index and sibling installations cannot redefine that inventory.
Missing roots, malformed inputs, empty required scopes and partial results fail
admission rather than producing a successful empty report.

The CI dispatcher owns arguments and execution; each check owns its exact input
set. Native formatter, schema and behavior failures retain their causes. A gate
requires writable progress output before it runs. A result-write failure after
publication reports the completed effect without repeating or compensating that
publication. Streaming and captured subprocesses share directory, environment
and explicit standard-input semantics; generic execution creates no build tree.

Three input adapters address demonstrated native-tool gaps:

- **TOML:** Taplo receives checkout-relative Git-selected TOML and lockfile
  paths. This prevents symlinked parent paths and extensionless lockfiles from
  disappearing through native include filtering.
- **Secrets:** Gitleaks receives one private regular-file projection because
  directory mode does not consume a Git file list. Paths and native policy are
  preserved; symlink targets and ignored untracked output are outside scope.
  A hidden operation directory keeps copied Go metadata outside package
  discovery. The owner removes it after success or failure.
- **Structured reports:** OpenSpec validates its official documents; its
  consumer requires a nonempty all-scope report with consistent totals and no
  findings from this checkout. The consumer does not recreate the parser.
  Native size and coverage reports similarly require complete, unique input
  attribution before their metrics can justify acceptance.

Architecture supports a Git-free source directory by walking the explicit root;
Git-selected link and source checks require a checkout. These are different
input contracts, not interchangeable fallback paths. Source, scratch and report
lifetimes follow [output ownership](../../../CONTRIBUTING.md#output-ownership-and-cleanup).

#### Native policy and schema owners

The Go toolchain owns language validation and platform selection. Architecture
checks own semantic package boundaries, import admission and carrier placement;
an allowed child does not require an unused package to exist. Every production
import allowance needs a current consumer. Go lint, SCC and coverage consume
their existing policies; CUE projects their command graph to both Forges.
Cross-repository consistency means equivalent responsibility boundaries, not
identical Go and Python directory layouts.

Each native tool validates its own configuration where it has that capability.
GoReleaser and golangci-lint schema checks precede execution. Markdownlint alone
does not reject unknown policy keys and invalid rule parameters, so
`tools/ci/markdown` binds the schemas bundled with the locked CLI and rule
packages to the maintained Go JSON Schema library. No schema copies, downloaded
references, second rule parser or additional executable are introduced. The
library is repository-only and absent from the product command dependency graph.

The quality tool graph must resolve under CUE's declared tool selection rather
than a developer's broader PATH. Native command resolution and Forge projection
conformance test the same declaration. Transitive tools and full native runs
retain their own execution requirements.

#### Structural limits and behavioral evidence

The [calibration decision](../../../docs/governance/change-and-release-policy.md#calibration-decision)
owns accepted limits and the reasons for retaining or tightening them. Every
trial includes product, tools, tests and platform-selected files. A lower score
is useful only when it reduces state, caller knowledge or verification cost
without weakening cohesive operations. Declaration tables, complete acceptance
journeys and ordered compensation are not split into shallow helpers to pass a
number. No exclusion, permanent offender baseline or hidden companion diagnostic
can stand in for that review.

Native Go owns statement counters. Coverage admission accounts for every
canonical package and rejects undeclared counters. Missing counters require
inspection of the selected source: only declarations without function bodies
establish a declaration-only package. Native zero-statement counters remain
not applicable, never an invented percentage. This is evidence admission, not a
parallel coverage analyzer.

Behavioral assertions compare complete owned observations where the contract
requires them. Independent native JSON readers preserve unknown-field detection;
fresh observations do not merge into preceding state. Scenario tables retain
separate inputs, effects and recovery assertions. Shared fixtures hide genuinely
shared setup, not a second behavior selected by scenario names or boolean modes.
Fault injection must still invalidate the corresponding assertion after a
simplification. Local conformance runs the actual check against boundary pairs,
preserves source bytes and distinguishes the intended rule from companion
failures. Current results belong only in the existing tasks and verifier output.

#### Documents, interfaces and trust

[Text layout policy](../../../docs/governance/text-layout.md) separates native
formatting, structural lint and semantic/rendered review. Every authored Markdown
location is covered by native discovery; OpenSpec archives retain their historical
role. Mermaid validation consumes the exact inventory through standard input and
uses the locked parser. Diagrams express actors, transitions and trust boundaries,
not implementation steps disguised as architecture. Browser layout and terminal
readability require independent observation of the final content.

Cobra owns command order and metadata; one renderer presents grouped and ordinary
commands without a parallel sorting path. Configuration admission owns stable
collection and reference order. Human and JSON readiness share the observed
facts and safe recovery, never inferring service identity from an address or
credential-read permission from metadata reachability.

Signed-object verification consumes the exact revision's complete subject policy
and the existing strict SemVer parser. Transport authentication cannot rewrite
object identity. ETHOS owns generic hooks and lifecycle transitions; AIGW owns
product policy, tests and CI. A defect in one does not justify recreating the
other or blocking independent product repair.

These layers support different claims: inventory does not prove check execution,
lint does not prove cohesion, compilation does not prove native operation, and
controlled streams do not prove a live Provider or final released artifact.
Final acceptance follows the existing execution order and
[completion evidence](../../../docs/governance/change-and-release-policy.md#performance-and-completion-claims).

### CUE owns CI semantics; Forges own syntax and capacity

One CUE graph describes facts and dependencies. GitHub Actions and GitLab CI are
generated projections. Jobs are separated by independently useful evidence:
fast quality, Go compatibility, native product journeys and published-artifact
verification. The existing release builder constructs and signs one immutable
matrix on the approved build host. Independent peer uploads consume that same
matrix; post-publication jobs download their own peer assets and verify public
trust, tagged source and provenance without signing keys. Platform proof follows real runner capability; superficial job
symmetry is not required, semantic parity is.

Final native qualification consumes the published matrix through the existing
`accept-native --artifacts` command. Public trust and provenance are checked
before execution; the product archive reader supplies the native executable in
owned scratch. Source-built acceptance uses that same reader and lifecycle.
The existing manual workflow selects the exact `candidate_tag` together with
the historical `baseline_tag`; rebuilding is not evidence about released bytes.

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
10. Integrate source only after its exact proof and required native checks pass,
    preserving the active Change for unfinished delivery. Synchronize selected
    local and Forge refs, publish the signed tag and identical assets, verify
    published bytes and install safely. Update the same tasks from those real
    outcomes; settle Change obligations before archive and exact final retirement.
