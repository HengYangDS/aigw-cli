# Design

## Context

AIGW 0.1.0 is an immutable published baseline. The repository already declares the intended product boundaries and many strong local and hosted gates, but accumulated delivery work can still leave physical topology, user journeys, tests, documentation, and quality policy harder to understand than the product requires. This Change converges those surfaces without rewriting the stable tag or importing generic lifecycle state into AIGW.

## Goals / Non-Goals

**Goals:**

- Establish one evidence-backed map from product journeys and invariants to semantic owners.
- Repair behavior before reorganizing its files, then make logical and physical ownership agree.
- Remove unconsumed entities and parallel semantics before adding tools or abstractions.
- Make setup, deferred activation, synchronization, credentials, client projection, installation, recovery, and extension natural on every supported platform.
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

Converge the paths in this order: configuration and route authority; credentials; client projections; installation and recovery; provider/client extension; repository topology; quality graph; documentation; performance and final acceptance. This order prevents structural refactors from preserving broken behavior and avoids running expensive matrices before local semantics stabilize.

### 4. AIGW and Proxy compose only through explicit endpoints

AIGW owns Accounts, Tokens, Profiles, Routes, and client projections. It carries no Proxy lifecycle, state, or mandatory loopback default. A profile may select a direct provider endpoint or any independently managed compatible endpoint. Proxy owns protocol translation and runtime traffic when explicitly installed. Tests use an external endpoint contract rather than importing Proxy implementation.

### 5. Quality has one declarative graph

The repository keeps one machine-readable mapping from tracked carrier classes to mature formatters, linters, analyzers, tests, security checks, and generated projections. GitHub and GitLab remain deterministic projections of that graph. Custom code is retained only for AIGW-specific semantics that general tools cannot express. Threshold changes require measured distributions and named risks, not aesthetic severity.

### 6. Cross-platform claims consume real released bytes

Source tests establish contracts; native jobs establish host behavior; published-artifact jobs establish distribution behavior. macOS, Linux, and Windows each exercise build, install, update, rollback, uninstall, credential mode, and client projection using the selected immutable release bytes. Unsupported platform trust, client availability, or credential service behavior remains explicit rather than inferred.

### 7. Documentation teaches by tracing the product

The root README remains the concise product entry point. Task-oriented guides explain complete user and contributor journeys; architecture documents explain stable boundaries; decisions record chosen trade-offs; research remains evidence for future choices. Code, commands, diagrams, tables, and links are validated through the same repository quality graph. No private local file may be a shared prerequisite.

## Risks / Trade-offs

- **Large scope can create churn** → complete one semantic closure at a time and require deletion plus focused acceptance before the next structural move.
- **Stricter gates can reward fragmentation** → derive thresholds from distributions and preserve coherent domain units.
- **Native evidence can become expensive** → run focused local falsification first, freeze inputs, then reuse exact matching immutable evidence.
- **External tools can expand the maintenance surface** → admit only stable tools that replace more code and operational burden than they add.
- **Breaking cleanup can surprise existing users** → remove only unsupported or unconsumed behavior; document migration for supported public contracts.

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

| Feedback theme                                                                                                                             | Owning tasks |
| ------------------------------------------------------------------------------------------------------------------------------------------ | ------------ |
| Natural first setup, partial credentials, absent clients, later synchronization, and precise `use`/`check` semantics                       | 2.1–2.6      |
| Keychain, Secret Service, Credential Manager, file and environment portability without repeated prompts                                    | 3.1–3.5      |
| Optional Proxy composition, direct endpoints, and low-cost Provider or Client extension                                                    | 4.1–4.6      |
| Semantic packages, test topology, precise names and types, no suffix-based flat sprawl, hard-coding, wrappers, or parallel implementations | 5.1–5.6      |
| Comprehensive format, lint, type, test, documentation, schema, security, complexity, size, coverage, and warning policy                    | 6.1–6.7      |
| Latest stable direct supply chain, locked clean-lane bootstrap, and removal of stale installers or caches                                  | 7.1–7.5      |
| English, navigable, accurate documentation; correct research, decision, architecture, guide, governance, and operations placement          | 8.1–8.6      |
| Real macOS, Linux, and Windows product journeys; performance; dual-Forge identity and CI projection                                        | 9.1–9.7      |
| Versioning, release only for changed product bytes, branch convergence, proposal cleanup, lane retirement, and residue removal             | 10.1–10.4    |

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
- after the stable delivery was accepted, both peer proposal refs were removed,
  local and peer `dev` converged on the signed object `b2fbec3a`, the superseded
  `stable-macos-release` Work Lane was retired, and this Change now owns the sole
  active Work Lane.

This is an ownership inventory, not a claim that every current package or file
is already optimal. Task 1.4 carries the consumer-level deletion audit; later
journeys may still prove that an apparent owner is redundant or misplaced.

## Accepted journey observations

Focused acceptance at the current baseline confirms the intended public
semantics before further restructuring:

- manifest setup imports capability with no Token and no installed client;
- any one available Account Token is sufficient, including an explicitly
  selected Account or the read-only environment backend;
- setup projects only the intersection of installed clients and usable Routes;
- installing a client or making its Token available later is completed by
  `aigw sync` or an explicit `aigw use <profile>`, without repeating setup;
- independent Claude and Codex selections remain independent, and `aigw check`
  accepts both without a hidden global or bulk-selection step;
- cancellation, invalid backend state, projection failure, persistence failure,
  and compensation failure preserve or restore the owned preimage and report the
  exact incomplete boundary.

The focused suites for CLI acceptance, onboarding, configuration,
synchronization, and secret backends pass at the current Work Lane base. These
observations establish the existing behavior for tasks 2.1 through 2.5. They do
not yet prove released-artifact execution on every host, which remains task 9.3.

The configuration-lifecycle audit found one public setup command, one shared
setup transaction, and no command alias or parallel persisted selection model.
The removed `recommended_default` manifest field and the preceding local
default-plus-overrides schema are rejected by strict decoding or exact version
admission; runtime code retains neither migration reader. Unsupported local and
manifest schema errors now state that AIGW does not reinterpret versions and
direct the operator to a matching release or an explicitly reviewed canonical
export instead of suggesting a generic readiness retry. Focused configuration,
presentation, onboarding, synchronization, and CLI acceptance suites pass.

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

The external-gateway boundary is accepted through tasks 4.1 to 4.5. Production
source and the shipped team manifest contain no Proxy identity, fixed Proxy
port, listener, service manager, installation, or runtime lifecycle. An Account
holds either a direct HTTPS endpoint or an explicitly selected loopback endpoint;
both follow the same Profile, Route, projection, readiness, and diagnostic path.
Loopback classification reports only that the service is external, while an
absent or unavailable endpoint produces the ordinary configuration or network
failure without installation, startup, retry, or repair of another product.
Historical Proxy-shaped test ports were replaced with a neutral loopback
fixture. Strict local and manifest schema admission rejects older versions and
unknown fields rather than retaining a compatibility reader or gateway field.

A synthetic `northstar` Provider passes parse, merge, connected-Account route
selection, runtime resolution, and client-native Codex projection using only
manifest data; no provider name enters the control-plane core. The sole
provider-specific production package remains the explicitly selected DMXAPI
Account diagnostic, which is outside Route and projection semantics. A
synthetic `future` Client passes the complete registry contract for discovery,
convergence, preflight, guarded projection, change detection, inspection, live
verification, compensation, disable, and withdrawal. The same registry rejects
unadmitted or incomplete implementations, prepares all selected clients before
writing, compensates in reverse order, and preserves existing Accounts, Routes,
and built-in clients. Adapter and uninstall acceptance confirm that withdrawal
removes only owned projection state and never the foreign client executable.
The extension and admission documents identify the same contract and keep
incompatible wire behavior in an independent data plane. Focused suites for
`internal/configuration`, `internal/client`, `internal/cli/acceptance`,
`internal/cli/adapter`, `internal/cli/install`, `internal/cli/readiness`, and
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
  Account/Profile/Route validation, explicit conflict admission, atomic store,
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
  credential, Route, or native-client projection ownership. It may be selected
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
sidecar-identity reader, an onboarding runtime selector, a route-list forwarding
function, a default-provider parsing wrapper, and a duplicate coverage-percent
helper. Their live safety properties remain exercised through the actual
`ValidateConfig`, `ReconcileConfigs`, route command, onboarding, and coverage
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
Account identifiers, labels, Tokens, Profile identifiers, Routes, endpoint
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
high-frequency runtime predicates; AIGW's explicit Profile and Route model has
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
projected environment credential helper, and 23.093 ms for one durable Route
projection. Its two configured-status peak-memory blocks each reached
14,876,672 bytes, compared with predecessor maxima of 14,827,520 and 14,925,824
bytes. Across the six release targets, executable-size change ranged from
-0.035% to +0.465%; the largest absolute increase was 50,080 bytes. All current
budgets pass, so no numerical gate changes merely to manufacture a tighter
score. This is current-source calibration, not the later multi-host published-
artifact acceptance owned by task 9.6.

Task 6.5 distinguishes repository failures from bounded external outcomes. The
exact-HEAD proof and complete macOS native entrypoint emit no repository-owned
warning and no skipped Go test. OpenSpec's remaining long-requirement `INFO` is
visible advice under the existing validation contract, not a warning or an
admission bypass. GoReleaser reports only its explicit snapshot exclusions:
publication and its disabled internal SBOM pipe are outside native acceptance,
while the `AIGW_BUILD_OS=darwin` selection excludes Linux and Windows artifacts
from that host-local build. CUE keeps those product/platform distinctions in
`productEvidence`, `forgeCapabilities`, and `nativeEvidence`; projection tests
prove that GitHub supplies all three native hosts and GitLab advertises only its
macOS and Linux capacity. The sole source-level `t.Skip` names the Windows
symlink-privilege limitation and executes the same ownership test on supported
hosts. No retry, ignored exit code, `allow_failure`, `continue-on-error`, or
silent capability promotion remains in the quality or Forge graph.

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

## Initial deletion inventory

The initial residue audit classifies current candidates before any removal:

| Candidate                                                             | Current classification                                               | Disposition                                                                   |
| --------------------------------------------------------------------- | -------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| RC.118 local executable, portable install root, and rollback copy     | Proved disposable after installed 0.1.0 acceptance                   | Already removed; retain no compatibility reader.                              |
| `proposal/engineering-reference-convergence` on both peers            | Review-only projection absorbed into exact `dev` object `b2fbec3a`   | Already removed by merged reviews.                                            |
| `work/20260919-stable-macos-release` and its worktree                 | Clean lane absorbed by accepted truth                                | Retired through ETHOS with exact-head receipt.                                |
| `.serena/` and `node_modules/` in the active Work Lane                | Ignored, reproducible development state                              | Keep only while the lane is active; remove with lane retirement.              |
| Git-common ETHOS runtime, attestations, receipts, and release records | Active governance runtime or durable evidence with current consumers | Preserve; lifecycle and retention policy belong to ETHOS, not AIGW.           |
| Signed release tags and OpenSpec archives                             | Immutable product chronology and release evidence                    | Preserve until a separate authorized retention policy proves them disposable. |

No tracked `evidence`, `claims`, `chronicle`, `.code-memory`, or `.ethos/state`
container exists. The repository currently exposes one work lane, one candidate
checkout, and no remote proposal branch. Further deletion remains scoped to the
semantic closure that proves a specific implementation, test, configuration,
or document has no consumer.

## Migration Plan

1. Freeze and inventory the current accepted product, published bytes, user journeys, tracked carriers, dependencies, and residue.
2. Repair and verify each product journey while retaining AIGW 0.1.0 as the working baseline.
3. Reorganize source and tests around the verified semantic owners; delete superseded material in the same closure.
4. Consolidate quality and CI projections, then upgrade direct dependencies under the complete graph.
5. Rewrite current documentation from the accepted design and verify navigation and rendering.
6. Run exact-HEAD, native, published-artifact, installation, performance, and residue acceptance; publish only a new version when product bytes change.
7. Archive this Change and retire its proposal and Work Lane through ETHOS after every task is evidenced.
