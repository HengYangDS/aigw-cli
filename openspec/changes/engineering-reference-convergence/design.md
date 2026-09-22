# Design

## Context

AIGW 0.1.0 is an immutable published baseline. The repository already declares the intended product boundaries and many strong local and hosted gates, but accumulated delivery work can still leave physical topology, user journeys, tests, documentation, and quality policy harder to understand than the product requires. This Change converges those surfaces without rewriting the stable tag or importing generic lifecycle state into AIGW.

## Goals / Non-Goals

**Goals:**

- Establish one evidence-backed map from product journeys and invariants to semantic owners.
- Repair behavior before reorganizing its files, then make logical and physical ownership agree.
- Remove unconsumed entities and parallel semantics before adding tools or abstractions.
- Make setup, deferred activation, synchronization, credentials, client projection, installation, recovery, and extension natural on every supported platform.
- Deliver Hermes and Claude Desktop integration, with explicit evidence for each supported desktop mode and host, and qualify model choice independently of client brand.
- Make the repository independently understandable and reproducible by a new contributor.

**Non-Goals:**

- Rebuild AIGW as a traffic gateway, daemon, desktop application, or package-manager framework.
- Add a tutorial subsystem, duplicate status ledger, compatibility facade, or local ETHOS state machine.
- Rewrite 0.1.0 artifacts, tags, historical OpenSpec archives, or user-owned client configuration.
- Change Codex Responses Proxy implementation from this repository.

## Decisions

### 1. Audit by semantic closure, not by directory

Begin with the public journeys and invariants, then trace their source, tests, configuration, documentation, and evidence. Each closure ends with one owner, focused tests, full gates where needed, updated documentation, and deletion of displaced material. A file-by-file cleanup without this trace is rejected because it can polish the wrong topology.

### 2. Delete before adding

For every duplicate helper, wrapper, configuration fragment, compatibility path, or document, first identify its current consumer and protected invariant. Delete it when both are absent. Reuse an existing owner when present. Add an entity only when no current owner can express the behavior without increasing coupling; record the displaced complexity in the existing decision register.

### 3. Product journeys define dependency order

Converge the paths in this order: configuration and Client Binding authority; credentials; client projections; installation and recovery; provider/client extension; repository topology; quality graph; documentation; performance and final acceptance. This order prevents structural refactors from preserving broken behavior and avoids running expensive matrices before local semantics stabilize.

### 4. AIGW and Proxy compose only through explicit endpoints

AIGW owns Accounts, Tokens, Profiles, Client Bindings, and native projections. It carries no Proxy lifecycle, state, or mandatory loopback default. A Profile may resolve a direct provider endpoint or any independently managed compatible endpoint. Proxy owns protocol translation and runtime traffic when explicitly installed. Tests use an external endpoint contract rather than importing Proxy implementation.

### 5. Quality has one declarative graph

The repository keeps one machine-readable mapping from tracked carrier classes to mature formatters, linters, analyzers, tests, security checks, and generated projections. GitHub and GitLab remain deterministic projections of that graph. Custom code is retained only for AIGW-specific semantics that general tools cannot express. Threshold changes require measured distributions and named risks, not aesthetic severity.

### 6. Cross-platform claims consume real released bytes

Source tests establish contracts; native jobs establish host behavior; published-artifact jobs establish distribution behavior. Before stable publication, macOS, Linux, and Windows exercise the immutable candidate through installation, update, rollback, uninstall, credentials, and client projection. After Change completion and archival, the release procedure qualifies the final distribution inputs and verifies the same published bytes through each selected peer. Any change to an actual build input requires renewed artifact evidence. Client platform availability and credential-service behavior remain explicit.

### 7. Documentation teaches by tracing the product

The root README remains the concise product entry point. Task-oriented guides explain complete user and contributor journeys; architecture documents explain stable boundaries; decisions record chosen trade-offs; research remains evidence for future choices. Code, commands, diagrams, tables, and links are validated through the same repository quality graph. No private local file may be a shared prerequisite.

### 8. Requested scope precedes current adapter inventory

Hermes and Claude Desktop are implementation obligations of this Change. A
synthetic Adapter proves that an interface can be extended; it cannot replace
either requested integration. The operational registry continues to describe
only implemented and admitted clients while the tasks retain incomplete work.
Claude Desktop Chat, Cowork, and Code have distinct feature evidence, and its
third-party inference configuration is independent of Claude Code settings.

Client, provider, model, protocol, and host surface are separate dimensions.
AIGW selects explicit endpoints and credential references through the shared
Client Binding and transaction owners. Client Adapters encapsulate native configuration,
authentication delivery, discovery, verification, and withdrawal. Model names
are provider data; usable capabilities come from the actual client and endpoint.
Protocol translation remains at an independently selected compatible service.
Provider mappings must identify the real serving model rather than imply that
a Claude- or GPT-shaped alias proves its identity.

OpenCode, Pi, CodeBuddy CLI, WorkBuddy Desktop, Qoder CLI/IDE, and ChatGPT
Chat/Work/Codex receive bounded source-backed dispositions. Their assessment
does not promise an unimplemented Adapter or silently remove Hermes and Claude
Desktop from delivery. AWS Bedrock and other provider recipes distinguish
native client authentication from compatible gateway composition. The existing
[research assessment](../../../docs/research/provider-tooling-assessment.md#client-surfaces-and-cross-model-inference)
owns source evidence; tasks own implementation progress.

Task 4.10 closes the Provider-extension decision with one three-way contract.
An ordinary compatible endpoint and Token are Account/Profile data. A client
that already owns authentication and request signing uses a Client Binding with
`authentication = "client-native"`; AIGW projects no credential helper. A true
wire mismatch belongs to an independently selected data plane. The existing
AWS regressions exercise import, selection, projection, readiness, diagnostics,
credential omission, and withdrawal for Codex's `amazon-bedrock` provider.
Current OpenAI and AWS documentation confirms the two native credential paths
and distinguishes recommended `bedrock-runtime` from compatibility
`bedrock-mantle`. Pricing remains a dated model, Region, and tier input rather
than portable manifest data. No AWS entitlement or live inference is claimed;
that remains a route-specific acceptance obligation rather than an Adapter
implementation requirement.

Task 4.9 closes the requested client-surface assessment without creating
placeholder Adapters. OpenCode, Pi, and CodeBuddy CLI expose plausible
noninteractive projection boundaries and remain future candidates. WorkBuddy
Desktop and Qoder IDE remain manual composition because their reviewed
interfaces do not provide an independently owned credential-safe write path;
Qoder CLI requires an official noninteractive import contract. ChatGPT Codex
mode shares the admitted Codex configuration boundary, while regular Chat,
voice, and Work do not. Each later implementation still requires its own
installed-client and host acceptance rather than inheriting this research
result.

### 9. Redesign intent before extending adapters

The requested outcome is choosing a usable model for a client, not maintaining
separate copies of the same model for every executable. The prior v3
`Profile.Client`, `Routes`, and `Adapters` structure is now bounded migration
input, not normal-runtime authority. Its costs were observable: a new client
changed unrelated readiness suggestions, protocol selection leaked into
credential probing, and each Adapter repeated selection and activation state.

The v5 runtime has three configuration concepts, not another layer:

- **Account:** provider endpoint declarations and one credential reference.
- **Profile:** one Account and real upstream model, independent of client brand,
  plus its explicitly verified protocol set and optional catalogue tier.
- **Client Binding:** selected Profile, explicit enabled intent, native target,
  authentication, protocol, and only genuinely client-specific options.

A Profile may be selected by several compatible clients. The client-specific
binding resolves the protocol from the intersection of the Profile's verified
protocol set, its Account endpoints, and the Adapter's supported interfaces.
Select the sole compatible endpoint automatically; ask for an explicit choice
when several remain. An omitted protocol set preserves manually authored
Profiles that have not claimed qualification; reviewed team Profiles always
declare it. Never guess from a model prefix, client brand, or declaration order.
Client-native authentication stays inside that client's contract rather than
acquiring a second AIGW token owner.

Current source, tests, public commands, and documentation use this replacement.
Stable installed state remains untouched until a candidate and the reviewed
migration pass native acceptance.

#### User journey

1. Import a token-free team catalogue or connect one Account. Other Accounts
   and absent applications are optional; no credential access is required to
   inspect the catalogue.
2. Choose a client surface and a compatible Profile. An explicit choice owns
   the default for future work, not the model of an existing conversation.
3. Resolve the native target, show the exact owned changes, and apply through
   the shared guarded transaction. Missing applications produce deferred
   activation, not placeholder files or failure of another client.
4. Synchronize only enabled client intent. Discovery observes availability;
   it never re-enables an explicitly disabled client or silently selects an
   alternative model because a credential is temporarily unavailable.
5. Status defaults to configured clients, explains saved versus applied versus
   verified state, and offers optional discovery separately. Local inspection,
   endpoint authentication, and quota-consuming inference remain distinct
   operations with explicit names and consequences.
6. Disable restores owned configuration while retaining the user's reusable
   Profile and explicit disabled intent. Removal withdraws the binding;
   uninstall removes only owned installation/projections and follows explicit
   credential retention policy.

The guided path and noninteractive commands must express the same choices.
`use` requires a client when the Profile does not determine a unique intended
binding; it must not silently affect every compatible installed application.
An explicit multi-client operation names its affected set before mutation.
Team imports never overwrite local selections or reactivate a disabled client.

#### Developer boundary

Keep a small Adapter contract around native observation, projection planning,
and real verification. The existing transaction owner applies and compensates
typed file changes; no new Adapter gets its own rollback coordinator. Format
readers preserve unrelated keys and reject ambiguous owned structures. Native
helpers, environment references, and SDK authentication precede wrappers or
additional credential processes. Provider data cannot select an application
configuration path or acquire session/service ownership.

A new provider with an existing protocol changes catalogue data. A new client
adds its native contract and acceptance tests. A genuinely missing protocol
uses a selected mature endpoint implementation before product-specific traffic
code is considered. A dependency is justified by a necessary capability or
removed maintenance responsibility, not by novelty.

#### Migration and deletion

Change the schema once for the coherent replacement. Parse an old version only
inside a bounded, explicit migration operation; normal runtime accepts the new
schema alone. The migration previews Accounts, equivalent Profiles, client
bindings, retained selections, native options and credential references. It
never copies secret values or changes native session metadata. Commit only
when all affected preimages still match; retain the immutable predecessor and
its guarded rollback input until candidate acceptance.

Delete the replaced `Profile.Client` selector, parallel Route/Adapter state,
client-name branches in shared orchestration, and duplicate rollback machinery
in the same semantic closure as their replacements. Do not ship aliases or two
runtime readers to avoid finishing migration. Preserve client-specific options
where they carry real behavior instead of forcing superficial uniformity.

#### Implementation dependency order

First prove the revised domain model and migration with retained-state tests.
Then implement one shared intent-to-projection transaction and migrate existing
Codex/Claude consumers. Next connect Hermes and Claude Desktop through that
same boundary, consuming the already established native contract tests. Finish
the team manifest, user/contributor guides and cross-platform native journeys
before the existing final qualification and archive/release steps. A green
fixture or checked historical task does not admit the new schema or client.

#### Lessons retained from CC Switch CLI

The current CC Switch CLI was re-read at commit
`1ab2882d89fac9ae0281f5937528252babd9af49` and stable release `v5.10.5`.
Its useful lessons are behavioral rather than structural:

- one explicit application selector makes the affected native surface clear;
- live configuration is not created for an application that has not initialized
  its own state;
- provider writes preserve unrelated native fields and reject stale preimages;
- discovery, switching, one-off launch, health checks, and data management are
  distinct user intents;
- shared frames, keymaps, and generated help keep the TUI internally
  consistent.

AIGW adopts the first four principles through its client binding and guarded
projection owners. It does not copy CC Switch's SQLite database, TUI, optional
proxy, session management, MCP, skills, prompts, synchronization service, or
usage subsystem. Those facilities serve a broader personal workbench product
and would add authorities outside AIGW's narrower team-catalogue and native
configuration contract. A future visual shell may consume AIGW's public
machine-readable commands, but it cannot become another configuration owner.

## Risks / Trade-offs

- **Large scope can create churn** → complete one semantic closure at a time and require deletion plus focused acceptance before the next structural move.
- **Stricter gates can reward fragmentation** → derive thresholds from distributions and preserve coherent domain units.
- **Native evidence can become expensive** → run focused local falsification first, freeze inputs, then reuse exact matching immutable evidence.
- **External tools can expand the maintenance surface** → admit only stable tools that replace more code and operational burden than they add.
- **Breaking changes can surprise existing users** → replace supported contracts when net benefit justifies it, with explicit migration, retained-state acceptance and guarded rollback before cutover.

## Accepted baseline

Subsequent comparisons use the immutable stable release rather than an RC or a
mutable checkout:

- source commit: `0e4c411410b264ab587aa90b8d237acc5e06fa79`;
- signed tag object: `ccd4853751fad10fc851207c5a348d4f8915b2c0`
  (`v0.1.0`);
- release inventory: the ten entries signed by `checksums.txt.sig`, including
  native archives for macOS, Linux, and Windows;
- peer identity: GitHub and GitLab expose the same tag object, peeled commit,
  checksum manifest, signature, and provenance bytes;
- installed product: Homebrew Cask `aigw 0.1.0` at
  `/opt/homebrew/Caskroom/aigw/0.1.0/aigw`, accepted as a notarized Developer ID
  application;
- retained user state: Keychain storage is available, Claude selects
  `ucloud-claude-fable-5-1`, Codex selects `ucloud-gpt-6-astra`, and current
  `status`, `check`, and `doctor` observations pass;
- transition evidence: GitHub workflow `35445819802` exercised
  `0.1.0-rc.118` to `0.1.0`, rollback, and forward recovery on macOS, Linux,
  and Windows; published-artifact verification passed in GitHub jobs
  `35445069751`, `35445072366`, and `35445075077`, and GitLab pipeline `7538`.

The preceding RC executable, portable installation directory, and rollback copy
are absent. Historical signed release records remain chronology, not an active
installation or compatibility path.

## Feedback acceptance map

The Change keeps one task ledger while mapping the accumulated feedback to its
owning closure:

| Feedback theme                                                                                                                                                                                         | Owning tasks or boundary |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------ |
| First setup, token-free team import, partial credentials, absent clients, deferred installation, later sync, and uninstall                                                                             | 2.1–2.3, 9.3             |
| `use`, per-client defaults, `use --all`, status, check, doctor, test, verify, recovery, cancellation, and migration UX                                                                                 | 2.4–2.8                  |
| Keychain, Secret Service, Credential Manager, file and environment modes, bounded noninteractive access, retained callers, and secret hygiene                                                          | 3.1–3.5, 9.3             |
| Optional Proxy composition, direct endpoints, endpoint identity, and removal of Proxy-shaped defaults from AIGW                                                                                        | 4.1–4.3                  |
| Provider-neutral protocols, non-OpenAI and non-Anthropic model families, exact serving-model identity, and synthetic plus real extension evidence                                                      | 4.4, 4.8, 4.10           |
| Hermes and Claude Desktop, including Chat, Cowork, Code, native discovery, restart, withdrawal, and client-owned state                                                                                 | 4.5, 4.7–4.8, 9.4        |
| OpenCode, Pi, CodeBuddy, WorkBuddy, Qoder, ChatGPT surfaces, CC Switch, local gateways, and mature-framework reuse                                                                                     | 4.6, 4.9–4.10            |
| Real team Profiles, Flagship and Daily model tiers, protocol metadata, channel variants, naming consistency, and preserved explicit selections                                                         | 4.11                     |
| Semantic packages, test topology, narrow names and types, deep modules, no suffix sprawl, hard-coding, wrappers, duplicate implementations, or stale residue                                           | 5.1–5.6                  |
| Comprehensive format, lint, type, test, docstring, documentation, schema, security, complexity, size, coverage, warnings, and deterministic Forge projection                                           | 6.1–6.7, 9.5–9.7         |
| Latest stable direct supply chain, repository-locked bootstrap, Work Lane environments, cache ownership, and cross-host reproduction                                                                   | 7.1–7.5, 9.3             |
| English, navigable, accurate documentation; correct research, decision, architecture, guide, governance, operations, diagram, table, and link ownership                                                | 8.1–8.6                  |
| macOS, Linux, and Windows builds, credentials, clients, installation, update, rollback, recovery, uninstall, performance, and portable artifacts                                                       | 9.1–9.7                  |
| Developer review versus maintainer closeout, proposal cleanup, signed object parity, main/dev convergence, release gating, Homebrew, optional later distribution channels, and repository housekeeping | 9.2, 9.5, 10.1–10.4      |

Generic Work Lane, lease, commitment, publication, review, and retirement
mechanisms remain ETHOS responsibilities. Proxy protocol translation, service
supervision, replay correctness, and host-service cleanup remain Proxy
responsibilities. Codex Desktop rendering and conversation-model selection, and
operator-wide private configuration, remain application or workstation
responsibilities. AIGW tests only its explicit boundary with each of those
systems and does not copy their state machines into this repository.

## Initial semantic inventory

The first repository-wide inventory establishes the surfaces that later closures
must preserve or deliberately remove:

- the public command graph contains one `aigw` composition root and the setup,
  selection, readiness, credential, client, installation, recovery, catalogue,
  and provider-administration journeys listed by `aigw --help`;
- the production graph contains 43 Go packages. Its principal dependency
  boundaries run from CLI adapters into configuration, presentation, process,
  secrets, synchronization, and transaction owners; the architecture gate
  currently reports no undeclared package or import edge;
- focused behavior is co-located with its semantic owner, while cross-command,
  client, upgrade, and release journeys live in explicitly named acceptance
  packages rather than in a second implementation;
- repository configuration has distinct owners for architecture, coverage,
  dependency policy, Go analysis, size, Markdown, secret scanning, TOML,
  release construction, and the CUE CI graph. GitHub and GitLab workflow files
  are generated projections of that CI graph;
- current documentation has one index and distinct architecture, concepts,
  decisions, experience, governance, guides, operations, research, history, and
  legal domains; every current document below `docs/` has an inbound tracked
  link;
- the tracked repository contains no `evidence`, `claims`, `chronicle`,
  `.code-memory`, or `.ethos/state` directory. Serena state and installed Node
  packages are ignored Work Lane-local development state;
- proposal refs and branch convergence are live delivery projections, not design
  facts; task 10.2 verifies their exact identities and task 10.3 removes the
  merged proposal refs and retires this Work Lane.

This is an ownership inventory, not a claim that every current package or file
is already optimal. Task 1.4 carries the consumer-level deletion audit; later
journeys may still prove that an apparent owner is redundant or misplaced.

## Accepted journey observations

Focused acceptance at the current baseline confirms the intended public
semantics before further restructuring:

- manifest setup imports capability with no Token and no installed client;
- any one available Account Token is sufficient, including an explicitly
  selected Account or the read-only environment backend;
- setup projects only the intersection of installed clients and usable bindings;
- installing a client or making its Token available later is completed by
  `aigw sync` for an existing enabled binding or an explicit
  `aigw use --for <client> <profile>`, without repeating setup;
- independent Claude and Codex selections remain independent, and `aigw check`
  accepts both without a hidden global or bulk-selection step;
- cancellation, invalid backend state, projection failure, persistence failure,
  and compensation failure preserve or restore the owned preimage and report the
  exact incomplete boundary.

The focused suites for CLI acceptance, onboarding, configuration,
synchronization, and secret backends pass at the current Work Lane base. These
observations establish the existing behavior for tasks 2.1 through 2.5. They do
not yet prove released-artifact execution on every host, which remains task 9.3.

Task 2.3 is accepted at signed commit `71da0af8`. A native installed-program
journey imports one four-client catalogue before any client is present, adds
Claude Code, Claude Desktop, Codex, or Hermes afterward, and synchronizes only
that client without another import. It compares every unrelated Client Binding
and projection byte-for-byte, then verifies uninstall removes the newly owned
projection. GitHub review run `35637431188` passed the four cases on native
macOS, Linux, and Windows; GitLab pipeline `7748` independently passed quality
and its available macOS and Linux native jobs. These fixtures prove each host's
discovery and projection surface. They do not substitute for real-client
inference, Claude Desktop Chat/Cowork/Code qualification, or published-artifact
acceptance, which remain tasks 4.7, 9.3, and 9.4.

The configuration-lifecycle audit found one public setup command, one shared
setup transaction, and no command alias or parallel persisted selection model.
The removed `recommended_default` manifest field and the preceding local
default-plus-overrides schema are rejected by strict decoding or exact version
admission; runtime code retains neither migration reader. Unsupported local and
manifest schema errors now state that AIGW does not reinterpret versions and
direct the operator to a matching release or an explicitly reviewed canonical
export instead of suggesting a generic readiness retry. Focused configuration,
presentation, onboarding, synchronization, and CLI acceptance suites pass.

Tasks 2.6 through 2.8 are accepted on the current Work Lane. Normal runtime now
loads only configuration schema v4; the explicit migration command is the sole
v3 reader. Its dry run preserves the exact configuration bytes, credentials and
client files. Apply writes one Client Binding model, retains the exact v3 bytes
as rollback input and rejects a changed preimage; rollback restores those exact
bytes while leaving credentials and client state untouched. The obsolete
`route` and `adapter` commands, their duplicate list state and their package
topology were deleted. `use --for`, `status`, `check`, `doctor`, `test` and
`verify` now consume one Client Binding authority; JSON uses one `clients` state
tree and manifest setup reports `selected_bindings` without a compatibility
field. `mise run check` passes with 95.34% statement coverage, and the macOS
`mise run native` gate passes retained-configuration rollback, delayed client
activation, partial-credential setup, portable lifecycle and the shipped team
manifest journey. These results do not establish the pending Linux, Windows,
or Claude Desktop acceptance tasks.

Task 4.5 admits Hermes through the same Client Binding and transaction owners
as the existing clients. Setup may retain enabled intent before Hermes is
installed; later discovery and synchronization select its native
`config.yaml`, while explicit re-enablement retains that target. Projection
groups every connected Account's reviewed, compatible Profiles into
deterministic native providers by Account and protocol, publishes their explicit
model allowlists with discovery disabled, and keeps the explicit Client Binding
as the one active provider/model. Disconnected Accounts and unverified manual
Profiles are omitted. Each projected provider receives its own Account-scoped
credential command rather than a Token. The transaction preserves unrelated
YAML and session files, rejects a changed preimage, and removes only AIGW-owned
providers on disable or uninstall. Verification uses Hermes' documented bounded
single-turn `chat` contract. The real Homebrew Hermes Agent `v0.21.3` executed an
authenticated streaming request against the selected Anthropic-compatible
endpoint, then passed Account rename, external credential-helper replacement,
disable, re-enable, and uninstall in an isolated home. This is macOS evidence;
Linux and Windows native qualification remain tasks 9.3 and 9.4.
The current native catalogue acceptance additionally loads two connected
Accounts through the installed Hermes runtime, observes every compatible
reviewed Profile under its Account-and-protocol provider, and proves that an
unconnected Account is not projected.

Task 4.7 now has a source-level Claude Desktop Adapter using the product's
per-user `Claude-3p/configLibrary` boundary on macOS, Linux and Windows. It owns
one stable UUID configuration, the corresponding metadata entry,
deployment-mode values and a compact ownership sidecar; it preserves unrelated
JSON fields, rejects managed drift, compensates partial writes and removes only
its own state. The team manifest recommends the same UCloud Fable Profile
independently for Claude Code and Claude Desktop. An isolated macOS run
discovered the installed Claude Desktop 2.2553.1 executable, imported the
shipped manifest, projected three compatible UCloud models, executed the
projected environment-backed credential helper, and withdrew every AIGW-owned
Desktop file without touching the real user configuration.

GitHub review run `35602717964` exposed a test-isolation defect rather than a
second Claude Desktop product path. Quality passed, Linux missed the repository
coverage floor at 94.77%, and the macOS and Windows native jobs could not find a
Desktop configuration library. The release fixture had installed every client
as a PATH executable even though Desktop discovery uses an application bundle
on macOS and `%LOCALAPPDATA%/AnthropicClaude/claude.exe` on Windows. The local
macOS run had therefore borrowed the operator's real `/Applications/Claude.app`
and concealed the missing fixture contract. Platform paths now have one owner,
discovery consumes that owner, and the native fixture creates a test-owned
executable at the selected host-native location. Host-native discovery and
fixture-isolation regressions reject both ambient application borrowing and
foreign-platform path creation. Focused native manifest acceptance passes all
four manifest cases, and the complete local quality graph passes at 95.22%
statement coverage. Hosted macOS, Linux, and Windows reruns remain required
before this evidence closes Task 4.7 or the platform obligations in Task 9.

The installed Claude Desktop 2.2553.1 schema independently identifies Chat,
Cowork, and Code as configurable third-party surfaces. The AIGW-owned profile
enables all three explicitly instead of inheriting release-specific client
defaults. Native diagnostics on 2026-09-22 exposed that the former profile ID
`aigw` violated Claude Desktop's UUID contract: the application reported an
applied four-character non-ID, retained the persisted `3p` choice, but resolved
effective mode `1P` with no inference provider. Reusing the same profile under
the stable UUID `6500fbf3-029c-5c0d-842a-48ee47e228c5` changed the real
application to `app://localhost`, exposed `UCloud · Claude Fable 5.1`, completed
an authenticated Cowork response and ran a Code session with tool activity. The
Adapter now migrates the invalid owned identity only after validating its
ownership hash, removes the superseded files, and retains guarded rollback and
withdrawal. Client enablement reports the projection as configured rather than
active and requires a restart; withdrawal reports the same boundary. Standalone
Chat and Linux and Windows host consumption remain required before Task 4.7 can
close.

Credential portability is accepted at signed commit `3863e05e`. The ordinary
GitHub review run `35455496791` exercised the complete native graph on macOS,
Linux, and Windows; Windows also completed the real Credential Manager journey.
GitLab review pipeline `7573` independently passed macOS and Linux after its
stopped OrbStack runtime was restored without changing repository code. The
focused stores verify explicit environment, file, and keyring selection,
read-only environment credentials, persisted automatic selection, precise
Secret Service unavailability, no fallback after an explicit keyring failure,
bounded credential subprocess termination, restricted identity environment, metadata-only
observation, and standard-input-only mutation. GitHub workflow `35457062367`
then consumed published `v0.1.0-rc.118` as the predecessor and passed the real
macOS Keychain create, read, rotate, update, rollback, forward-recovery,
uninstall, reinstall, and exact-delete journey. These observations establish
tasks 3.1 and 3.2; Linux proves the unavailable Secret Service path rather than
claiming a service that the runner does not provide.

Task 3.3 is accepted at signed commit `84f1edb3`. GitHub workflow
`35462782608` consumed published `v0.1.0-rc.118` and built the exact candidate
source. Its disposable macOS Keychain journey exercised setup, create, read,
rotate, staged identity migration, verified finalization, update, rollback,
forward recovery, retained Claude and Codex credential commands, uninstall,
reinstall, and exact deletion. Configuration bytes and the selected backend
remained unchanged across replacement. Current-HEAD provider-double suites
independently pass postimage-guarded replacement, backend-choice, setup, route,
account-migration, and file-recovery failure cases without accessing operator
credentials. Accepted `dev` run `35463197605` then passed the macOS, Linux,
Windows, and quality jobs at the same commit. The candidate executable differs
from the published `v0.1.0` artifact, so this is transition evidence, not a
claim that current HEAD has already been released.

Task 3.4 retains a narrower boundary than its original shorthand suggested.
The default projected credential command is owned by the active AIGW
executable. The supported `credential_command` field remains an explicit,
operator-owned extension contract: AIGW preserves it but neither installs,
discovers, owns, nor falls back to that executable. Cleanup therefore targets
only AIGW-owned duplicate identities, automatic readers, stale owned slots, and
unsupported compatibility paths.

The task 3.4 consumer and host audit finds one native implementation dependency,
`go-keyring`, behind the cross-platform bounded credential subprocess. The current macOS
Keychain exposes exactly the `aihubmix`, `dmxapi`, and `ucloud` slots under
service `AIGW_TOKEN`, matching the three configured Accounts; no test or stale
slot remains. The rejected `aigw-keychain`, `aigw_keychain.py`, and
`keychain-return` executables, source references, and launchd identities are
absent. Both installed Claude and Codex projections invoke
`/opt/homebrew/bin/aigw`, which resolves to the signed Homebrew 0.1.0 binary,
and an installed `aigw sync --dry-run --json` reports both targets already
converged. The independent `keychain-blob` executable remains outside AIGW: its
credential-governance source and tests are active consumers, and neither AIGW
configuration nor client projection selects it.

Task 3.5 uses the repository's pinned Gitleaks policy rather than a second
scanner. The current authored tree, local recovery records, GitHub runs
`35462782608` and `35463197605`, GitLab pipeline `7591`, and the recursively
opened `v0.1.0` assets from both peers contain no finding; all 12 peer assets
also have identical SHA-256 values. A reachable product-ref history scan finds
one false positive consisting of two synthetic model identifiers in an old
test. Extending the scan to all refs adds 63 false positives from ETHOS's
private attestation-set ref: 53 Git object digests and 10 structured Git object
references. The three current artifact-store findings are the public SSH
signing status. Explicit private-key and common access-token patterns are
absent. Source review confirms that projection fingerprints hash only client,
Account, and endpoint identity; artifact and ownership hashes consume program
or non-secret projection bytes, not Tokens. Current Claude sidecars record no
present original credential value, and the first-adoption path rejects
plaintext credentials or a foreign helper before writing a sidecar.

The external-gateway boundary is accepted through tasks 4.1 to 4.3. Production
source and the shipped team manifest contain no Proxy identity, fixed Proxy
port, listener, service manager, installation, or runtime lifecycle. An Account
holds either a direct HTTPS endpoint or an explicitly selected loopback endpoint;
both follow the same Profile, Client Binding, projection, readiness, and diagnostic path.
Loopback classification reports only that the service is external, while an
absent or unavailable endpoint produces the ordinary configuration or network
failure without installation, startup, retry, or repair of another product.
Historical Proxy-shaped test ports were replaced with a neutral loopback
fixture. Strict local and manifest schema admission rejects older versions and
unknown fields rather than retaining a compatibility reader or gateway field.

A synthetic `northstar` Provider passes parse, merge, connected-Account binding
selection, runtime resolution, and client-native Codex projection using only
manifest data; no provider name enters the control-plane core. The sole
provider-specific production package remains the explicitly selected DMXAPI
Account diagnostic, which is outside Client Binding and projection semantics.
A candidate-built AIGW 0.2.0 and native Codex CLI 0.155.1 also completed one
authenticated Responses stream for each AIHubMix Flagship and Daily Profile:
the 12 reviewed Grok, Gemini, DeepSeek, Qwen, GLM, and Kimi models. The fixture
rejected an unknown model, a missing credential, non-streaming input, or effort
other than `high`; each explicit `verify --profile` used an isolated projection
and preserved the selected Client Binding byte for byte. This evidence proves
the declared compatible protocol and exact requested model, not the upstream
vendor's serving identity or broader tool, cancellation, continuation, and
compaction behavior, which remain task 4.8.

Task 4.11 closes on the canonical version 6 [`team.toml`](../../../manifests/team.toml),
whose SHA-256 is `71540fec516305bc84407b5a0d802a679c617335939d0dab15dae8310b40413f`.
It contains three Accounts, 70 credential-free Profiles, and one independent
recommendation for each admitted client. Profile identifiers retain the exact
Account and upstream model spelling; labels use `Account · Model [· Channel]`,
DMXAPI channel variants remain explicit, and catalogue entries omit subjective
purpose text. Each general model family has one declared Flagship and Daily
Profile per Account with an explicit verified protocol. The native team journey
builds the current candidate, imports the manifest with no Token, then connects
each Account independently through the environment backend, synchronizes all
four admitted clients, preserves every Profile, proves a second sync byte-stable,
and removes only owned state. Focused configuration, CLI acceptance, and native
release suites pass. Live inference breadth remains task 4.8 rather than being
inferred from manifest admission.

A synthetic `future` Client passes the complete registry contract for discovery,
convergence, preflight, guarded projection, change detection, inspection, live
verification, compensation, disable, and withdrawal. The same registry rejects
unadmitted or incomplete implementations, prepares all selected clients before
writing, compensates in reverse order, and preserves existing Accounts, Client Bindings,
and built-in clients. Adapter and uninstall acceptance confirm that withdrawal
removes only owned projection state and never the foreign client executable.
The extension and admission documents identify the same contract and keep
incompatible wire behavior in an independent data plane. Focused suites for
`internal/configuration`, `internal/client`, `internal/cli/acceptance`,
`internal/cli/client`, `internal/cli/install`, `internal/cli/readiness`, and
`internal/codex` pass with these boundaries.

Task 4.6 compares replaceable responsibility rather than feature count. At this
closure, AIGW declares 14 direct Go modules; the selected build list contains
63 modules and the compiled command graph contains 315 packages. The installed
arm64 program is 10,770,864 bytes. The relevant product owners contain 1,448
production lines in configuration, 912 in client orchestration and adapters,
and 211 in optional Provider diagnostics. These counts bound the surface a
candidate could displace; they do not by themselves justify retention or
adoption.

The bounded upstream review on 2026-09-20 produced these decisions:

- [koanf 2.3.6](https://github.com/knadh/koanf/releases/tag/v2.3.6) declares
  three direct and one indirect core module requirement; [Viper
  1.21.0](https://github.com/spf13/viper/releases/tag/v1.21.0) declares ten
  direct and seven indirect requirements. Both own generic source loading and
  merging. Neither removes AIGW's strict schema, unknown-field rejection,
  Account/Profile/Client Binding validation, explicit conflict admission, atomic store,
  or source-preserving client projection. Their alias, default, watch, or
  ambient-source behavior would introduce a second configuration semantic, so
  neither is adopted.
- [go-plugin 1.8.0](https://github.com/hashicorp/go-plugin/releases/tag/v1.8.0)
  declares seven direct and eight indirect requirements and adds subprocess,
  RPC, handshake, version, installation, trust, logging, and cleanup contracts.
  The static in-process registry already centralizes selection, preflight,
  compensation, inspection, verification, and withdrawal; a dynamic plugin
  layer would delete none of the client-specific ownership work. It is rejected
  unless independently shipped third-party adapters become a demonstrated
  product requirement.
- [Fx 1.24.0](https://github.com/uber-go/fx/releases/tag/v1.24.0) declares six
  direct and three indirect requirements, while [Wire
  0.7.0](https://github.com/google/wire/releases/tag/v0.7.0) is archived. AIGW's
  explicit construction has no unresolved dependency-graph or component
  lifecycle problem, so either dependency would add machinery without deleting
  a product obligation.
- [LiteLLM 1.101.0](https://github.com/BerriAI/litellm/releases/tag/v1.101.0)
  is a Python SDK and traffic gateway, not a replacement for local Account,
  credential, Client Binding, or native-client projection ownership. It may be selected
  as an external endpoint, or evaluated later against Proxy with protocol and
  lifecycle evidence, but it must not enter AIGW as a framework dependency.

The resulting decision is to add no framework. Existing narrow dependencies
already delegate TOML parsing, native credential APIs, bounded Windows cleanup,
and CLI grammar at their exact owners. Reconsider this decision only when a
candidate proves a required behavior and deletes more implementation,
verification, platform, and operating responsibility than it introduces.

Task 5.1 binds the production topology to executable evidence rather than a
directory impression. The baseline mapped by that task contained 45 production packages. Every
one appeared exactly once as an import owner in the architecture policy, with no
stale product owner, and both declared composition roots match their production
files exactly. The architecture gate reports no undeclared package, child,
composition-root file, import edge, carrier class, semantic name, or Decision
Record defect; the complete Go package list also resolves successfully. The
dependency direction matches the product concepts documented above:
configuration, credentials, guarded transactions, process execution, discovery,
presentation, and readiness are stable capabilities; Client owns admitted
client orchestration; provider-specific code is confined to optional
diagnostics; synchronization and CLI packages compose those owners; upgrade and
repository tools remain independent entry points. The indexed source graph has
complete parse coverage for product code and reports one internal validation
recursion cycle, not a package dependency cycle. Focused architecture, CI, and
release tool suites pass against this same topology.

The final Task 5.1 publication binds that topology to a product-owned
`architecture.claim-model/v2` and `architecture.edition/v1` under
`docs/architecture/client-projection/`. The selected Claim Model covers 14 exact
source owners at product revision `dcfb274c4cb5bc8c87d9d1a21867768ea39cffdc`;
Architecture Publisher package `0.2.0-alpha.0` with SHA-256
`f853ae1fb149ef1329fd28e5230c7f76101eeab4d63d8ec81b6b3324b272f2f3`
compiled candidate
`b2035f2bde00b790aa1b6e9854d5420e51fa1d1f7a361f538b81d61bb57599b7`.
The source bundle manifest is
`455f20c4c7285fccdf05faeeb6025d4aa56b901df721cc409857590186627414`
and the candidate manifest is
`7abb2c3a1d33d44fccc0148332dd5eefcd4a5496eb1a746e376ce5c784c0d266`;
the Edition, static scene, and interactive scene digests are respectively
`c1b7368908923d63e9fae77e4ca00aa6b4d6277f3408cec25f2a6eab0f1c82d2`,
`445f374686b71c10d52b713a25b2125fef996786f9ff01ff1196d58e2c29927a`,
and `29f15c7116857d78d427a3b6687126c3a1c33543fd6877cc687def694874eea2`.
A network-disabled installation into an empty relocated HOME reproduced the
manifest and every member byte exactly. A native Chrome run loaded the overview
without page errors, exposed named keyboard-focusable controls, honored reduced
motion, changed theme, and focused one semantic node. The static overview was
independently rendered and visually reviewed. Generated media remain ignored
acceptance output; AIGW adds no Publisher runtime, wrapper, or second command
plane.

The first physical simplification removes the one-file
`internal/client/acceptance` test-only subpackage. Its credential-projection
journeys exercise the public Client contract but own no separate implementation,
fixture API, or package boundary; they now live as external tests beside
`internal/client`, and the obsolete child declaration is removed from the
architecture policy. Test-only acceptance packages for public CLI and release
journeys remain because they span multiple production owners and represent
distinct product boundaries rather than suffix-based mirrors of one package.

The same review found that `internal/verification` had no consumer outside the
Client Adapter owner. Its live Codex and Claude invocation contract and tests
therefore move intact to `internal/client/verification`; `internal/client`
continues to own selection, isolated projections, credential suppression, and
the public Adapter result. This removes a top-level semantic owner and import
edge without changing the client process plan, timeout, response marker,
redaction, or cleanup behavior. The complete internal test graph, Go lint, and
architecture gate pass after the move.

The terminal-capability code formerly exposed as `internal/console` has one
production consumer and changes for the same reason as human rendering: output
width, interactivity, colour, and Windows virtual-terminal support. It now lives
inside `internal/presentation`, preserving its platform variants and focused
tests while deleting the shallow package and the CLI-to-console edge. Stable
surface identity remains separate because Client, Codex, and CLI owners all
consume it; collapsing that boundary would increase coupling rather than remove
accidental complexity.

Task 5.2 completes the production-layout review at 44 packages. Every remaining
directory names a product capability, command boundary, or reusable release
responsibility; no concatenated package name remains. Native `_darwin`, `_linux`,
`_unix`, `_windows`, and `_test` suffixes are Go's platform and test selection
contracts, not substitute domains. Configuration, secrets, Codex projection,
upgrade, synchronization, and renaming remain cohesive deep modules because
their private files coordinate one transaction or invariant; splitting those
files would expose more state without reducing a caller's burden.
`internal/client/verification` remains a deliberate child because it isolates a
complete quota-consuming protocol with its own timeout, redaction, subprocess,
response, and cleanup contract. `internal/surface` remains a neutral identity
boundary shared by Client, Codex, and CLI owners, while
`internal/providers/diagnostic` prevents the optional Provider registry and its
implementations from forming an import cycle. The review therefore deletes the
one proved shallow package without manufacturing replacement subpackages.

Task 5.3 applies the same ownership test to verification code. Package-local
white-box tests remain beside the private invariant they exercise; external
`*_test` packages exercise public contracts. The CLI acceptance package remains
one cross-command product boundary over configuration, credentials, Clients,
discovery, prompting, and upgrade, and its shared fixtures are private test-only
adapters rather than a second implementation. Upgrade acceptance likewise owns
the public peer-resolution, transport, candidate, and installation journey.
The Client credential-policy tests already belong beside `internal/client`; the
misleading `credentials_acceptance_test.go` name is replaced by
`credential_policy_test.go`. No shared test library or additional test package
is introduced.

Repository quality execution no longer exposes unused npm-script aliases for
formatting, Markdown, OpenSpec, or signature checks. `package.json` now owns
only the locked Node dependency declaration, while the existing Go CI command
remains the sole quality command plane and invokes the repository-local OpenSpec
executable and npm's native signature audit directly. This deletes four unconsumed entry points and one unnecessary
Node-to-npm forwarding hop without changing the quality graph.

Task 5.4 closes the repository-tool topology. `mise.toml` and its locks own
bootstrap; `.config/checks` contains concern-specific policy only; `tools/ci`
owns the executable quality graph and deterministic CUE projection;
`tools/forge` owns Git-object trust and peer publication; and `tools/release`
owns release readiness, construction, native acceptance, artifact validation,
and release publication. The former generic `tools/repository` command had only
Changelog chronology and release-epoch consumers, duplicating parsing already
inside release construction. Those contracts and their adversarial tests now
live once under `tools/release/readiness`; CI calls the release owner directly,
and the obsolete command, package, documentation path, and architecture entry
are deleted. The surviving tool packages and configuration files all have
current code, task, CI, documentation, or release consumers; projection drift,
tool bootstrap, Changelog/tag binding, strict semantic-version ordering, and
release construction tests pass after the consolidation.

Task 5.5 removes implementations retained only by tests after their product
consumers disappeared: the standalone Codex inspection result model and
sidecar-identity reader, an onboarding runtime selector, a route-list command,
function, a default-provider parsing wrapper, and a duplicate coverage-percent
helper. Their live safety properties remain exercised through the actual
`ValidateConfig`, `ReconcileConfigs`, selection command, onboarding, and coverage
paths. A cross-platform `deadcode` 0.50.0 audit over Darwin arm64, Linux amd64,
and Windows amd64 reports no unreachable functions when all declared acceptance
build tags are enabled. The production-only view leaves eleven intentional test
seams: credential backend constructors, direct Codex projection helpers, and
native release construction. Direct Go and Node dependencies each retain a
current source or tool consumer; `go mod tidy -diff` is empty. No non-ignored
untracked or empty directory remains. Active-lane `.serena` and `node_modules`
remain reproducible workspace inputs until lane retirement; branch and Work Lane
removal remain task 10.3 rather than being performed beneath active work.

Task 5.6 narrows the remaining domain language around the objects AIGW actually
owns. The guided `add` journey now connects an Account and its first Profile;
optional provider-platform credentials live under `account diagnostics`, so
they cannot be confused with an Account Token or with connectivity itself.
Account identifiers, labels, Tokens, Profile identifiers, Client Bindings, endpoint
runtimes, and native credential services retain distinct names in code and
human output. The former generic renaming `Service` is now `Renamer`, while
Codex and Claude projection transitions use closed action types at their owning
boundaries. The Client Adapter aggregate intentionally keeps its action as a
string because future admitted adapters form an open set; converting each
client's private enum at that boundary avoids a false shared enumeration.

The same audit enables `recvcheck` for observational receiver consistency with
one documented exception: `Config.Normalize` is the sole intended mutator.
Repository-wide scans find no remaining public `add <service>`, first-service,
account-connect, account-disconnect, or current-service wording. Legitimate
uses of `service` remain only where the subject is an external runtime or a
native credential service. The WorkBuddy/Qoder review preserves the same
precision: Desktop, CLI, and SDK surfaces are assessed independently, and
documented custom-model support is not reported as AIGW Adapter admission.
Focused internal tests, Go lint, strict OpenSpec validation, and whitespace
validation pass after the closure; the complete repository graph is the final
acceptance prerequisite before the task is marked complete.

Task 6.1 makes the executable quality graph explicit without copying it into
architecture policy. The existing architecture policy remains the sole owner
of the fourteen tracked carrier classes and their selectors. `tools/ci` now
owns twenty-seven named executable gates, the ten supported quality concerns,
and the carrier-to-gate coverage relation. Before returning any `quality`,
`source`, or full native sequence, the existing CI entrypoint compares that
relation with the live architecture carrier inventory and rejects missing or
unknown classes, unknown or unused gates, and required concerns without an
executing gate. The same graph supplies the executable command sequences. This
preserves the specification boundary: architecture proves one
semantic owner, while executed native gates prove format, lint, type, test,
security, architecture, documentation, schema, workflow, and projection
coverage. The inventory covers the complete tracked tree, including immutable
OpenSpec history through common byte, secret, and ownership gates; archive
formatting remains deliberately excluded by its existing immutable-history
policy. Focused CI and architecture tests plus the repository Go quality gate
pass with the new graph.

Pants, Dagger, Nix, and CEL are not admitted by this closure. None displaces a
current AIGW responsibility without adding another build, execution,
environment, or policy authority. CUE 0.17.1 remains the stable locked CI model
because it already replaces separately maintained Forge workflows. CEL may be
reconsidered only for a future data-plane product that needs operator-authored,
high-frequency runtime predicates; AIGW's explicit Profile and Client Binding model has
no such requirement. Pants, Dagger, and Nix require the same future test: remove
more owned execution and environment complexity than they introduce, without
weakening native macOS, Linux, or Windows evidence.

Task 6.2 confirms one authority per quality concern. Concern-specific native
policy files under `.config/checks` are consumed directly by their owning implementation or
mature tool; none is a second command registry. The `repositoryQualityGraph`
is the only executable source and the former `qualityCommands` value is now a
derived compatibility-free view used by existing internal tests and native
composition. `.config/ci/pipeline.cue` remains the single hosted topology and
renders `.gitlab-ci.yml`, `.github/workflows/verify.yml`, and
`.github/workflows/release.yml`; byte-level reconciliation rejects edits to any
projection without modifying it. Both Forge quality jobs invoke the same locked
`go run ./tools/ci quality` entrypoint and declare the same tool closure.
Focused projection tests, exact projection reconciliation, and the complete
repository quality graph pass at signed commit `7b6e496e`.

Task 6.3 adds only two mature checks that close measured gaps. Typos 1.50.2
receives the exact tracked and current checkout inventory through its native
file-list interface; its policy excludes immutable OpenSpec archives and names
only the official `importas` analyzer identifier. The first full scan found one
test-only split word, which the fixture now constructs without retaining a
misspelling in source. ShellCheck 0.11.0 is a locked dependency of the existing
actionlint invocation, not another shell command plane; an injected unquoted
workflow variable proves that actionlint actually delegates to it. Both tools
have mise checksum and download bindings for Linux and macOS on arm64 and x64,
and Windows on arm64 and x64, and CUE projects the same tool closure to GitHub
and GitLab. Current YAML and JSON already receive formatting and semantic
validation from their native owners, so Yamlfmt or another generic parser would
duplicate authority rather than improve coverage.

Task 6.4 recalibrates the existing machine limits against the product tree at
`a1668b9b`. The locked analyzers inspected the complete package graph under
Darwin arm64, Linux amd64, and Windows amd64 selections; SCC independently
measured all 372 tracked and current Go files: 113 product files, 32 repository
tool files, and 227 test files. The trial made no exclusions and retained every
test assertion. Its result is deliberately not a mandate to split coherent
transactions or acceptance journeys:

| Trial                  | Current findings on each target selection                                   | Decision                                                                  |
| ---------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| SCC 450, then 400      | 36 files above 450; 61 above 400. Tests account for 35 and 53 respectively. | Keep 500; the largest product/tool files are 487/433 and tests reach 500. |
| Cyclomatic 20, then 15 | 50 at 20 on every target; 158 on Darwin and 157 on Linux/Windows at 15.     | Keep 25; lower limits primarily fragment complete behavioral tests.       |
| Cognitive 40           | 23 on every target: two product functions and 21 tests.                     | Keep 45.                                                                  |
| Span/statements 110/55 | Eight on every target: one product, three tools, and four tests.            | Keep 120/60.                                                              |
| Six parameters         | Four on every target: one product, one tool, and two tests.                 | Keep seven.                                                               |
| Nesting score four     | No current finding under the already adopted fail-at-four rule.             | Keep the current strict rule.                                             |
| Maintainability 30     | 17 on every target: one product, one tool, and 15 tests.                    | Keep 25.                                                                  |
| Duplicate threshold 80 | 13 test findings on Darwin/Linux; 15 including two Windows product pairs.   | Keep 100 and review matches by semantic ownership.                        |

The exact-HEAD repository proof reports 96.35% statement coverage
(10,436/10,831), above the single greater-than-95% floor with every canonical
package observed. A local macOS arm64 comparison then measured the current
source program against the downloaded published `0.1.0` predecessor. The
candidate's pooled p95 was 4.442 ms for version, 4.300 ms for help, 4.323 ms
for configured status, 4.331 ms for configuration export, 6.720 ms for the
projected environment credential helper, and 23.093 ms for one durable Client Binding
projection. Its two configured-status peak-memory blocks each reached
14,876,672 bytes, compared with predecessor maxima of 14,827,520 and 14,925,824
bytes. Across the six release targets, executable-size change ranged from
-0.035% to +0.465%; the largest absolute increase was 50,080 bytes. All current
budgets pass, so no numerical gate changes merely to manufacture a tighter
score. This is current-source calibration, not the later multi-host published-
artifact acceptance owned by task 9.6.

Task 6.5 distinguishes repository failures from bounded external outcomes. The
exact-HEAD proof and complete macOS native entrypoint emit no repository-owned
warning and no skipped Go test. OpenSpec's requirement-length guidance is
resolved by assigning delivery, credential, distribution, test-isolation, and
manual-qualification semantics to distinct canonical requirements.

GoReleaser reports only its explicit snapshot exclusions: publication and its
disabled internal SBOM pipe are outside native acceptance,
while the `AIGW_BUILD_OS=darwin` selection excludes Linux and Windows artifacts
from that host-local build. CUE keeps those product/platform distinctions in
`productEvidence`, `forgeCapabilities`, and `nativeEvidence`; projection tests
prove that GitHub supplies all three native hosts and GitLab advertises only its
macOS and Linux capacity. The sole source-level `t.Skip` names the Windows
symlink-privilege limitation and executes the same ownership test on supported
hosts. No retry, ignored exit code, `allow_failure`, `continue-on-error`, or
silent capability promotion remains in the quality or Forge graph.

The OpenSpec ownership split is verified through the official archive command
in an isolated copy of the repository state. The archive reports no lifecycle
warning, the resulting ten canonical specifications validate without findings,
all fourteen scenarios formerly nested under the overloaded delivery
requirement survive exactly once, and no requirement title is duplicated across
capabilities. Active Change deltas remain the sole pre-archive mutation source;
canonical specifications are not edited ahead of the governed archive.

Task 6.6 reuses the adversarial regressions already introduced at each semantic
owner instead of adding a parallel fault-test layer. Invalid command shapes are
rejected before storage creation; Account connection and secret replacement
exercise partial acquisition and compensation; compare-and-swap file writes and
multi-target projection preflight model concurrent external edits; registry,
synchronization, renaming, and process tests inject cancellation before and
during effects; bounded process tests retain stderr and report interrupted pipe
drain; Claude and Codex tests reject foreign ownership, malformed state, and
stale hashes; diagnostics and secret selection classify unavailable services;
and client, secret, synchronization, and program replacement tests preserve the
primary failure when rollback also fails. Focused execution of those owners
passes at `db864bcb`. The regressions exercise real public or domain boundaries,
not mocks of the assertion itself, and retain the failure-inducing fixtures that
would reject removal of the corresponding guard.

Task 6.7 closes the quality phase with one execution order. Each semantic change
first ran its smallest owning package or native adapter tests. The consolidated
repository graph then ran once after tasks 6.1–6.6 stabilized, including exact
projection reconciliation and the source-only package-observed coverage gate.
No retry, issue limit, new-only mode, baseline, source exclusion, or generated
workflow edit masked a failure. Exact-HEAD ETHOS proof remains the final local
admission for the signed commit; native and hosted release matrices retain their
separate task 9 obligations.

The accepted publication of `af25e36f` exposed one remaining CI duplication:
the publisher atomically advanced `main` and `dev` to the same object, but both
push events launched the complete platform graph. CUE now distinguishes
pipeline admission from evidence-producing job admission. Review, tag, manual,
and `dev` events retain the complete graph, preserving both developer review
and direct maintainer paths. A `main` push runs only accepted-ref parity, which
proves that the release and accepted refs name the same object without spending
a second platform matrix on identical source, locks, toolchain, and claimed
facts. Both Forge files remain generated projections of this decision.

## Supply-chain closure

Tasks 7.1–7.3 treat authored manifests as dependency truth and upstream release
metadata observed on 2026-09-20 as freshness evidence. Renovate retains its
three-day delay for unattended proposals. This maintainer-directed batch admits
newer stable inputs only after checking their published identity, immutable
digest or registry signature, compatibility, focused owner tests, and
deterministic lock output.

- **Language and package managers:** Go 1.27.1, Node 26.9.0, npm 12.0.2, and
  mise 2026.9.11. Exact pins and repository locks select execution; ambient
  fallback is disabled. AIGW has no Python manifest or Python runtime, so it
  declares no Python compatibility line.
- **Product Go modules:** `charm.land/huh/v2` 2.0.3,
  `charm.land/lipgloss/v2` 2.0.6, `Masterminds/semver/v3` 3.5.0,
  `charmbracelet/x/ansi` 0.11.8, `gofrs/flock` 0.13.1,
  `pelletier/go-toml/v2` 2.4.3, `rogpeppe/go-internal` 1.16.0,
  `santhosh-tekuri/jsonschema/v6` 6.0.3, `spf13/cobra` 1.10.2,
  `spf13/pflag` 1.0.10, `zalando/go-keyring` 0.2.8,
  `go.yaml.in/yaml/v3` 3.0.5, `x/sys` 0.48.0, and `x/term` 0.46.0. The exact
  module graph is governed by update discovery, tidy, integrity, test, race,
  and vulnerability checks.
- **Repository Node tools:** OpenSpec 1.13.1, Mermaid Lint 0.53.1,
  markdownlint-cli2 0.23.3, and Prettier 3.9.8. The exact package lock is
  exercised by deterministic installation, registry-signature, format,
  Markdown, Mermaid, and OpenSpec gates.
- **Release and Forge tools:** GitHub CLI 2.101.0, GitLab CLI 1.118.0,
  GoReleaser 2.18.2, Syft 1.52.0, OSV-Scanner 2.6.0, and Apple codesign 0.29.0.
  Native version probes, release tests, and signed artifact checks own their
  use.
- **Quality tools:** CUE 0.17.1, Taplo 0.10.0, SCC 4.1.0, EditorConfig Checker
  4.0.2, Gitleaks 8.30.1, golangci-lint 2.13.2, ShellCheck 0.11.0, actionlint
  1.7.12, Lychee 0.24.2, and Typos 1.50.2. Each has one existing quality-graph
  consumer; Hyperfine 1.20.0 remains scoped to performance acceptance.
- **Hosted inputs:** checkout 7.0.1, mise-action 4.3.0, and upload-artifact
  7.0.1 use immutable commits matching their official tags. Renovate 44.103.6
  and the mise 2026.9.11 Debian image use immutable multi-platform index
  digests. CUE remains the single authored owner of both Forge projections.

The admitted updates are markdownlint-cli2 0.23.3, EditorConfig Checker 4.0.2,
and Renovate 44.103.6. markdownlint-cli2 now owns `smol-toml` 1.8.0 directly,
so its former repository override was deleted. The remaining Mermaid override
has a current consumer: removing it resolves to deprecated `whatwg-encoding`
through jsdom 26, while the admitted jsdom 30 and whatwg-url 17 graph is warning-
free and passes the same nine-diagram validation.

The mise release introduced a breaking image-tag distinction after 2026.9.11
was published. The unqualified version tag now names a scratch image with no
shell; the official `-debian` variant is the runnable CI base. AIGW therefore
uses `2026.9.11-debian` at its verified multi-platform digest, derives the
GitHub action version from that one CUE value, and removes the old empty-
entrypoint override. Projection tests reject an unqualified image, a missing
digest, or any entrypoint patch. The complete lock was regenerated twice from
`mise.toml`; both results were byte-identical and contain no obsolete
`provenance_verified` compatibility fields.

GitLab merge-request pipeline 7620 first exposed a Linux ARM64 prerequisite
before any product gate ran: the locked Node 26.9.0 binary requires
`libatomic.so.1`, while the official Mise Debian image does not include that
library. A same-architecture Docker counterexample reproduced the failure with
the exact pinned image and lock, and Debian's `libatomic1` restored Node and npm.

The next same-source pipeline, 7627, proved that treating the first missing
library as the whole contract was incomplete. The `quality` job reached source
signature verification but the image lacked `ssh-keygen`; `native-linux`
reached the race gate but CGO was disabled, and a CGO-enabled Go toolchain also
requires a compiler and C development headers. These are not unrelated job
exceptions: they are the operating-system capability closure of the declared
quality and native graph. The CUE owner therefore declares the single minimal
Debian package set `gcc`, `libatomic1`, `libc6-dev`, `openssh-client`, and
`procps`: the last package supplies `ps` to the existing process-ownership
acceptance rather than making that test infer process state through a second,
platform-specific implementation. The model projects the package set once
through `.linux-toolchain` and explicitly enables CGO for the Linux quality and
native jobs. Projection tests reject an incomplete package set, missing CGO, or
job-local installation copies. No alternate image, entrypoint override, retry,
or second bootstrap owner is added. Task 7.4 remains open until the exact image
passes real ARM64 execution and the repaired hosted run, followed by the other
supported clean-host bootstrap journeys.

## Documentation closure

Tasks 8.1–8.6 retain one documentation index and the existing semantic domains:
architecture explains current structure, concepts define product language,
decisions preserve durable trade-offs, experience owns interaction, guides own
operator journeys, governance owns repository policy, operations owns Forge
procedures, and research informs later decisions without becoming policy. All
22 current documents below `docs/` have an inbound tracked link; no current root
or documentation page refers to an untracked local target or private host path.

The root README is reduced from 431 to 245 lines and now leads from installation
through first Account connection, deferred client activation, daily use,
boundaries, recovery, removal, contribution, and deeper documentation. The team
guide, security model, architecture boundary, Adapter admission policy,
contributor guide, and Forge operations retain the complete setup, credential,
extension, native-client, installation, and release journeys rather than copying
them back into the entry point. A built current executable returned valid help
for all 42 command and command-group surfaces used to check those examples.

The complete current Markdown graph passes Prettier, markdownlint, repository
policy, spelling, local-link, and Mermaid validation. A bounded online Lychee run
checked 352 links, 226 unique, with no error, timeout, unsupported target, or
redirect after updating moved upstream locations. The sole current documentation
diagram rendered through the installed Mermaid dependency to a valid SVG and was
visually inspected without clipped or overlapping labels. Every current entry,
architecture, concept, decision, experience, governance, guide, operations, and
research page also rendered through the locked Markdown parser with a leading
title and intact tables and fenced blocks. Distinct semantic paragraphs now use
one blank line; the text-layout owner records that rule without duplicating
Prettier's wrapping.

The contributor entry point now gives one bounded TDD path from requirement and
semantic owner through failing regression, minimal repair, focused verification,
complete source gate, and exact-range review. Existing source checks prove the
clean checkout reconstructs local Node dependencies and rejects ambient tool
fallbacks. Duplicate entry-point prose and stale release-detail copies were
removed; current navigation remains complete after that deletion.

## Initial deletion inventory

The initial residue audit classifies current candidates before any removal:

| Candidate                                                             | Current classification                                               | Disposition                                                                   |
| --------------------------------------------------------------------- | -------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| RC.118 local executable, portable install root, and rollback copy     | Proved disposable after installed 0.1.0 acceptance                   | Already removed; retain no compatibility reader.                              |
| Proposal refs for this Change                                         | Active review projections until final accepted integration           | Remove after their exact objects are admitted to `dev`.                       |
| `work/20260919-stable-macos-release` and its worktree                 | Clean lane absorbed by accepted truth                                | Retired through ETHOS with exact-head receipt.                                |
| `.serena/` and `node_modules/` in the active Work Lane                | Ignored, reproducible development state                              | Keep only while the lane is active; remove with lane retirement.              |
| Git-common ETHOS runtime, attestations, receipts, and release records | Active governance runtime or durable evidence with current consumers | Preserve; lifecycle and retention policy belong to ETHOS, not AIGW.           |
| Signed release tags and OpenSpec archives                             | Immutable product chronology and release evidence                    | Preserve until a separate authorized retention policy proves them disposable. |

No tracked `evidence`, `claims`, `chronicle`, `.code-memory`, or `.ethos/state`
container exists. Live lane, checkout, lease, candidate, and proposal topology
is observed at the operation boundary rather than copied into this design.
Further deletion remains scoped to the semantic closure that proves a specific
implementation, test, configuration, or document has no consumer.

## Migration Plan

1. Freeze and inventory the current accepted product, published bytes, user journeys, tracked carriers, dependencies, and residue.
2. Repair and verify each product journey while retaining AIGW 0.1.0 as the working baseline.
3. Reorganize source and tests around the verified semantic owners; delete superseded material in the same closure.
4. Consolidate quality and CI projections, then upgrade direct dependencies under the complete graph.
5. Rewrite current documentation from the accepted design and verify navigation and rendering.
6. Complete Hermes and Claude Desktop integration, cross-model qualification, the real team manifest, and bounded client/provider research before freezing the release candidate.
7. Complete current-HEAD governance, native candidate, real-client, credential, performance, and residue acceptance; verify every task and the official archive preview.
8. Archive the completed Change through ETHOS and integrate the archival commit through the reviewed branch path. Keep the active lane until these effects finish.
9. Follow the existing release procedure: qualify final build inputs, sign and notarize where required, publish identical assets to selected peers, update Homebrew, verify downloads and installation, and converge local and peer `main`/`dev` refs.
10. Retire the exact owned Work Lane and remaining disposable resources through their existing owners. Release records and native lifecycle results carry these post-archive effects; an archived task file is not rewritten to report later deployment state.
