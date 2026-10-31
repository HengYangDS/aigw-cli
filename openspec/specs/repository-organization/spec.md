# repository-organization Specification

## Purpose

Define the repository's product-version, lifecycle, semantic ownership,
documentation, and portable quality boundaries so every governed surface has
one discoverable authority.

## Requirements

### Requirement: One repository version source

AIGW SHALL expose one tracked, machine-readable product version source used by CLI version output, changelog validation, artifact naming, and release checks.

#### Scenario: Local build reads the product version

- **WHEN** a contributor runs the repository-native build or version check
- **THEN** the command reads the same tracked version source without requiring a Forge tag or personal environment variable

#### Scenario: Version metadata disagrees

- **WHEN** a tag, changelog heading, generated artifact, or build input disagrees with the tracked version source
- **THEN** the repository gate fails before packaging or publication

### Requirement: Governed release-branch convergence

Candidate, accepted, and release refs SHALL advance through current authority and the shared governed roles. Normal and maintainer transitions SHALL preserve proof, identity, compare-and-swap, and post-effect observation. Each Forge SHALL publish the same accepted source independently; local work SHALL require neither remote. OpenSpec SHALL remain progress authority until delivery is complete and archived.

#### Scenario: Accepted content is ready for release

- **WHEN** exact-head proof has admitted the candidate and accepted `dev` is current
- **THEN** the declared governed release transition advances `main` from that accepted content
- **AND** local readiness remains distinct from remote publication.

#### Scenario: Direct branch mutation is attempted

- **WHEN** an actor attempts to move a declared ref without current exact-scope authority or matching observed source and destination objects
- **THEN** admission blocks the mutation
- **AND** reports the missing authority or changed precondition
- **AND** a governor defect requires an explicit bounded maintainer recovery, not a permanent hook bypass or a replacement lifecycle engine.

#### Scenario: One Forge is unavailable

- **WHEN** one publication plane cannot be reached
- **THEN** the other may publish and verify the same signed revision independently
- **AND** local accepted state remains valid without either remote.

#### Scenario: A workspace omits retired role fields

- **WHEN** native adoption removes the retired branch-name overrides
- **THEN** the existing CI projection consumes the shared accepted and release roles
- **AND** the adopter has no competing configurable role policy or interpretation of ETHOS internals
- **AND** the current Forge output retains its event, branch, identity and admission semantics.

### Requirement: Portable repository quality surface

Repository configuration, tools, source, tests, documentation, and release assets SHALL have one semantic owner. Native tool settings SHALL have one consuming carrier; repository parameters SHALL have one owning policy. Every quality and release reader SHALL use its declared carrier and refuse absent or malformed required settings. Unrelated consumers SHALL own adaptation and qualification outside this product's implementation and quality graph.

#### Scenario: A contributor follows one behavior

- **WHEN** a contributor traces a public command or projection
- **THEN** implementation, tests, specification, and documentation identify one owner
- **AND** no compatibility facade or duplicate configuration must be consulted.

#### Scenario: Physical layout introduces competing authority

- **WHEN** root-level sprawl, forwarding scripts, concatenated package names, cross-package private calls, or duplicated policy create a parallel authority
- **THEN** the content is organized under its semantic owner
- **AND** those structures do not remain as a competing implementation or policy.

#### Scenario: Contributor uses another host

- **WHEN** the repository is verified on macOS, Linux, or Windows
- **THEN** repository-owned Go commands and declarative configuration provide the same contract
- **AND** shell-specific behavior is not required for product or CI correctness.

#### Scenario: An unrelated consumer reads product material

- **WHEN** an external product consumes selected repository material
- **THEN** its adaptation and output remain outside this repository
- **AND** AIGW qualification does not require that external product or its tests.

#### Scenario: A native configuration carrier moves

- **WHEN** an admitted migration moves tool settings to their consuming owner
- **THEN** every current native reader, fixture and documented entry uses that owner
- **AND** parsed settings, quality thresholds and generated outputs retain their accepted meaning
- **AND** immutable historical originals remain with their evidence consumers.

#### Scenario: A required formatter configuration is unavailable

- **WHEN** the native formatter fix or check command cannot read its declared settings
- **THEN** the command fails before changing any authored input
- **AND** it does not substitute ambient configuration or defaults.

### Requirement: Semantic documentation architecture

Documentation SHALL have one global entry point and semantic organization.
OpenSpec artifacts are the sole tracked change-intent authority; ETHOS
MAY derive only transient execution inputs through its public contract. The
adopter SHALL NOT persist ETHOS internals or parallel intent.
Filenames MUST identify subjects. Any otherwise necessary local index or carrier
MAY exist only for otherwise unrepresented semantics and MUST declare its
owner, consumer, displaced authority, and retirement.

#### Scenario: Reader enters the documentation

- **WHEN** a reader starts at `docs/README.md`
- **THEN** the entry point SHALL expose task-oriented paths and the complete
  information-domain map
- **AND** every canonical document SHALL be reachable from that map or a named
  semantic register.

#### Scenario: A document has a single semantic owner

- **WHEN** a document describes architecture, concepts, decisions, evidence,
  experience, governance, guidance, or operations
- **THEN** its directory and filename SHALL identify that owner
- **AND** no compatibility copy or redirect-only document SHALL remain.

#### Scenario: A directory contains multiple documents

- **WHEN** a semantic directory gains another document
- **THEN** file count alone SHALL NOT require a local `README.md`
- **AND** navigation SHALL remain with the smallest content-bearing owner.

#### Scenario: A repository gate consumes a semantic register

- **WHEN** a quality gate validates a documentation register
- **THEN** it SHALL consume the register's semantic filename
- **AND** it SHALL NOT require a container-named compatibility carrier.

#### Scenario: Governance evaluates change intent

- **WHEN** ETHOS evaluates the selected OpenSpec change
- **THEN** it SHALL compile the Commitment transiently from official OpenSpec
  artifacts
- **AND** the repository SHALL persist no parallel Commitment carrier.

#### Scenario: Historical change evidence is inspected

- **WHEN** a maintainer inspects an archived change
- **THEN** official OpenSpec archives and Git history SHALL describe the tracked
  change
- **AND** ETHOS Attestations SHALL remain the effect-evidence surface.

### Requirement: Release chronology shares strict version semantics

Release headings, tags, source admission, construction, and update admission
SHALL share one strict SemVer reader of canonical `VERSION`. Published headings
SHALL decrease strictly by precedence; exact tag and release-epoch lookup SHALL
retain non-ordering build metadata. Unsupported numeric values fail before
downstream work rather than silently overflow.

#### Scenario: Native acceptance receives an invalid source version

- **WHEN** `VERSION` is absent or does not conform to strict SemVer
- **THEN** native acceptance SHALL fail before invoking tests or artifact
  construction
- **AND** the same carrier reader SHALL own release construction and artifact
  journey identity without a weaker CI-specific grammar.

#### Scenario: A release includes build metadata

- **WHEN** a published heading and its selected tag include build metadata
- **THEN** admission SHALL require their complete version identities to match
- **AND** release-epoch lookup SHALL identify that exact heading.

#### Scenario: Several headings share version precedence

- **WHEN** published headings differ only by build metadata
- **THEN** chronology validation SHALL report that the order is not strictly
  descending rather than invent an order from metadata.

### Requirement: Logical and physical ownership are isomorphic

Source, tests, repository tools, configuration, documentation, schemas, and
generated projections SHALL be organized by stable product or repository
responsibility. Names SHALL be precise, readable words; flat suffix families,
concatenated compounds, ambiguous buckets, and implementation-shaped groupings
SHALL be replaced by cohesive semantic packages when they conceal ownership.

#### Scenario: Several files implement one responsibility

- **WHEN** a file family shares one lifecycle, dependency direction, and reason
  to change
- **THEN** it resides under one named semantic owner
- **AND** callers do not import sibling implementation details.

#### Scenario: A carrier has no current consumer

- **WHEN** a file, directory, compatibility path, evidence shell, generated
  projection, runtime, branch, tag, or configuration entry protects no current
  invariant and has no current consumer
- **THEN** it is deleted rather than retained for history
- **AND** no compatibility alias or forwarding wrapper is added.

### Requirement: Each Work Lane reconstructs its own mutable environment

Every Work Lane SHALL reconstruct its own mutable dependency and build state
from committed locks through one repository entrypoint. Work Lanes MAY share
only immutable or content-addressed caches and MUST NOT share another lane's
virtual environment, dependency tree, build output, or temporary state.

#### Scenario: A fresh Work Lane is created

- **WHEN** a contributor enters the Work Lane without `.venv`, `node_modules`,
  build output, or test state
- **THEN** one documented bootstrap command reconstructs the complete declared
  development environment
- **AND** no ambient interpreter, global configuration, or sibling checkout is
  required.

#### Scenario: A Change updates the toolchain

- **WHEN** one Work Lane changes a lock or tool version
- **THEN** only that Work Lane's mutable environment is rebuilt
- **AND** shared content-addressed caches remain safe for other lanes.

### Requirement: Repository navigation follows reader intent

Every public document and governed directory SHALL have one intentional entry
path, meaningful links to adjacent concepts and operations, and no empty index
whose only purpose is structural symmetry.

#### Scenario: A reader follows a named concept

- **WHEN** documentation refers to another internal document, command, decision,
  or external standard
- **THEN** it uses a valid navigable link when a stable target exists
- **AND** the link text states the destination's meaning rather than its file
  name alone.

#### Scenario: A contributor shares a local reference

- **WHEN** a document links to a local file or directory
- **THEN** the link check SHALL require that target to resolve to tracked
  repository content, including a staged addition
- **AND** an ignored or untracked file present only on the author's machine
  SHALL NOT satisfy the check
- **AND** current product and research documents SHALL cite shared primary
  sources or published evidence rather than private verification output.

#### Scenario: A document names an authority without a link

- **WHEN** a document presents a specification, configuration, decision, code
  owner or procedure as an authority or next step
- **THEN** semantic review SHALL verify a navigable link at its introduction
- **AND** literal examples and runtime paths SHALL remain distinguishable from
  references to tracked artifacts.

### Requirement: Repository meaning is traceable through one semantic path

Every public behavior, invariant, configuration rule, and extension point SHALL have one discoverable semantic owner linking its specification, implementation, tests, operational guidance, and quality enforcement. A contributor SHALL NOT need a compatibility alias, filename convention, private local artifact, or duplicate policy carrier to reconstruct that meaning.

#### Scenario: A contributor traces a product behavior

- **WHEN** a contributor starts from a public command, user journey, or documented invariant
- **THEN** repository navigation SHALL lead to one owning package or tool, its focused tests, its canonical specification, and its operational documentation
- **AND** sibling files SHALL not implement an undisclosed parallel path.

#### Scenario: Physical organization contradicts semantic ownership

- **WHEN** flat suffix families, ambiguous directories, mixed responsibilities, or misplaced documents conceal one reason to change
- **THEN** the repository SHALL move the content to the smallest cohesive semantic owner
- **AND** delete superseded paths rather than retain forwarding shells or compatibility copies.

### Requirement: A clean checkout reproduces the contribution path

A supported contributor SHALL reconstruct each Work Lane's mutable development state from committed locks through one documented repository entrypoint. The resulting environment SHALL expose the same format, lint, type, test, security, architecture, documentation, and generation contracts used by CI without depending on a sibling checkout or ambient system package version.

#### Scenario: A new contributor changes one invariant

- **WHEN** a contributor starts from a clean checkout and follows the documented bootstrap and verification path
- **THEN** the declared environment SHALL be reconstructed without manual dependency discovery
- **AND** the contributor SHALL be able to locate, change, test, and review one semantic owner using repository commands.

### Requirement: Decision records share one rationale structure

Each decision record SHALL declare its record identity and an accepted
disposition in its front matter and SHALL hold exactly the sections Context,
Decision, Alternatives Rejected, Consequences and Boundary, and Evidence and
Revisit, each with authored rationale and no lower headings. A record SHALL
NOT carry execution content such as code blocks, task lists, inline HTML or
command invocations.

#### Scenario: A decision record is validated

- **WHEN** the repository's governance reads a record under `docs/decisions`
- **THEN** its front matter SHALL name the identity that matches its file
  sequence and the accepted disposition
- **AND** its body SHALL hold the five rationale sections in order.

#### Scenario: A record needs to name a command

- **WHEN** a decision depends on a product command
- **THEN** the record SHALL describe the command's behavior in prose and leave
  its usage to the guides instead of carrying the invocation.

### Requirement: Repository declarations follow the shared paradigm

The repository SHALL NOT restate a role or policy that the shared ETHOS
paradigm already owns. Its release mirror SHALL be declared in the release
declaration. Commit-provenance verification SHALL require the shared SSH
signature policy when the repository declares no commit policy, and a declared
commit policy MAY only restrict commit scopes.

#### Scenario: A workspace table restates signing

- **WHEN** a tracked commit policy declares its own signing requirement or
  format
- **THEN** commit-provenance verification SHALL refuse that policy.

#### Scenario: No commit policy is declared

- **WHEN** the verified commit carries no workspace table
- **THEN** verification SHALL apply the shared subject grammar and SSH
  signature requirement to every commit in the range.

### Requirement: Generated output stays outside the Go source graph

The Go module SHALL declare its generated-output root as outside its source
graph. Package patterns, static analysis, tests and coverage SHALL then match
only product and tool source in every checkout, including a canonical checkout
that retains evidence.

#### Scenario: Retained evidence contains Go source

- **WHEN** the ignored generated-output tree holds captured Go files from
  different packages
- **THEN** `go vet ./...`, `go list ./...` and the repository's quality graph
  neither load nor report them
- **AND** no evidence file has to be moved, renamed or deleted for a proof to
  pass.

### Requirement: Product packages form one downward layer stack

The architecture gate SHALL declare product packages and repository tools as
one ordered stack of layers. A package's production files SHALL import only
its own descendants, the seams that the policy declares for its subtree, and
packages in lower layers. A repository tool's production files SHALL import
product packages only below the use-case layer. Test files are fixtures and
stay outside the stack.

#### Scenario: An import points upward or sideways

- **WHEN** a production file imports a package in its own or a higher layer,
  or one client adapter imports another
- **THEN** the architecture gate fails and names the importing package, the
  imported package and both layers
- **AND** no per-edge allowance can admit the import.

#### Scenario: A package joins a declared layer

- **WHEN** a package is added under a declared layer's path prefix and imports
  only lower layers
- **THEN** the architecture gate admits it without a policy change.

### Requirement: Client knowledge stays in its client package

Product code that names an admitted client or encodes its native files,
settings, commands or protocols SHALL live in that client's package. Outside
it, only the composition root that registers adapters and test fixtures SHALL
name a client.

#### Scenario: A shared package names a client

- **WHEN** a product package outside a client's package contains an identifier
  or string literal that spells that client's ID
- **THEN** the architecture gate fails with the path, the line and the client
  ID.

#### Scenario: A client's native value contains another client's ID

- **WHEN** a client's own native value contains another client's ID, such as
  the Hermes API mode `codex_responses`
- **THEN** the architecture policy declares that exact value as a native term
  of the first client
- **AND** the gate accepts it only as that literal in that client's package and
  fails when the declared term no longer occurs there.

#### Scenario: A client is added

- **WHEN** a test-only rehearsal adapter is registered
- **THEN** its change touches only its own package, the test composition root
  and its fixtures.

### Requirement: Command packages mirror the command tree

Each top-level command SHALL be implemented by one package named after it.
Acceptance scripts SHALL run the program in its own process for every
command's help, a representative success, a representative refusal and its
JSON output on macOS, Linux and Windows.

#### Scenario: A command has no package or script

- **WHEN** a top-level command is registered without a package of its name or
  without acceptance scripts
- **THEN** the repository gate fails and names the command.

#### Scenario: Command output changes

- **WHEN** a change alters a command's help, exit status or JSON output
- **THEN** that command's acceptance script fails until the same change
  updates the script.

### Requirement: One canonical name per concept

The product concept glossary SHALL give each concept one canonical term and
its rejected synonyms. Code identifiers, package names, specifications and
documentation SHALL use the canonical term, and no two production packages
SHALL share a base name.

#### Scenario: A rejected synonym appears

- **WHEN** a tracked code, specification or documentation file uses a declared
  rejected synonym
- **THEN** the text gate fails with the path, the line and the canonical term.

#### Scenario: A package base name repeats

- **WHEN** two product or tool packages share a base name
- **THEN** the architecture gate fails and names both package paths.

### Requirement: Main specifications stay within one reading unit

Each main specification SHALL own one capability and stay within the
repository's document line limit. Moving a requirement to another
specification SHALL keep every one of its scenarios.

#### Scenario: A specification exceeds the limit

- **WHEN** a main specification exceeds the document line limit
- **THEN** the source-size gate fails and names the specification and its
  line count.

#### Scenario: A requirement changes owner

- **WHEN** a change moves a requirement to another capability
- **THEN** the locked merge keeps all of its scenarios under the new owner
- **AND** the previous specification no longer states it.

### Requirement: Local release builds write under build/artifacts

The repository's local release build SHALL write its candidate to a directory
named for the version and commit under `build/artifacts/`, and SHALL write no
release output to any other repository path.

#### Scenario: An operator builds a release candidate

- **WHEN** an operator runs the declared release build task
- **THEN** the candidate appears under `build/artifacts/<version>-<commit>/`
- **AND** no `dist/` directory is created at the repository root.

### Requirement: Portable source

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, signing key, fingerprint,
signing program, trust anchor, foreign-application private path, or external
service lifecycle. CI SHALL provide trust material only from protected context.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, host, or
  Forge
- **THEN** build, verification, setup, repair, and uninstall SHALL not require
  the original contributor's machine, account, key, IDE, or workstation state

#### Scenario: An operator uses the installed command

- **WHEN** another team installs AIGW on a supported host
- **THEN** the installed executable SHALL provide the user command surface
- **AND** repository tools SHALL remain developer-only entrypoints
- **AND** documented configuration, credentials and discovery inputs SHALL not
  require an author-specific path, identity, key, service or foreign product.

#### Scenario: An operator selects environment credentials

- **WHEN** `AIGW_SECRET_BACKEND=env` is selected
- **THEN** the documented Account variable mapping SHALL supply credentials
  without persistence
- **AND** the user guide SHALL explain process inheritance and direct installed
  command invocation without exposing real credentials.

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner. Composition roots SHALL
assemble declared owners; source gates SHALL enforce positive package topology,
dependency direction, public surfaces, portability, and the canonical coverage
policy. Compatibility facades and duplicate policy owners SHALL not be retained.

#### Scenario: semantic ownership regresses

- **WHEN** a change violates declared topology or dependency direction, or misses the canonical coverage policy
- **THEN** verification SHALL fail with the exact semantic owner and evidence gap.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change violates declared semantic ownership or dependency direction, or misses the canonical package or aggregate coverage policy
- **THEN** local and hosted verification SHALL fail before publication.

#### Scenario: Foreign-host absolute path enters policy

- **WHEN** policy contains an absolute or parent-traversing path in another host's syntax
- **THEN** validation SHALL reject it identically on macOS, Linux, and Windows.

#### Scenario: No admitted branch authority exists

- **WHEN** no maintained admitted analyzer can measure a proposed branch metric
- **THEN** the quantitative policy SHALL omit that unsupported claim
- **AND** native statement evidence and complete package observation SHALL remain
  enforced without being relabeled as branch coverage.

#### Scenario: A tool needs shared release policy

- **WHEN** repository release tooling and product update read the same release artifact format
- **THEN** both import that format from its one product owner
- **AND** each still validates its own authority-bound inputs: the release tool its source and lock, product update its trust anchors and downloaded bytes.

#### Scenario: A legacy concatenated name remains

- **WHEN** a package appears outside the declared direct-owner topology or a repository tool imports a product package outside its admitted layers
- **THEN** the architecture gate fails with the exact path and dependency.

#### Scenario: an ordinary provider is added

- **WHEN** a provider is added below the existing provider owner without changing topology
- **THEN** the existing positive topology admits it without changing repository-shape policy.

### Requirement: Deterministic local verification

Local verification MUST use controlled fixtures rather than an undeclared
public network dependency or host-provided Git templates and hooks, and it
SHALL leave no temporary directory of its own behind. A native qualification
that needs public services SHALL run only in a quality gate that declares
network access.

#### Scenario: A local test depends on external state

- **WHEN** a verification test would contact a public endpoint instead of its
  controlled fixture
- **THEN** local verification SHALL fail or the fixture SHALL intercept the
  exact request without public network I/O

#### Scenario: A host provides Git templates

- **WHEN** the host configures a Git template directory with hooks,
  configuration or exclusions
- **THEN** repositories created by verification tests SHALL contain none of
  that content

#### Scenario: A host provides global Git hooks

- **WHEN** the host's global Git configuration names a hook directory
- **THEN** verification tests SHALL NOT execute those hooks
- **AND** fixtures SHALL keep their own hook directories and the host's other
  global settings

#### Scenario: A native tool needs public services

- **WHEN** a test qualifies a native tool against public advisory or license
  services
- **THEN** it SHALL run only in a quality gate that declares network access
- **AND** offline behavior tests SHALL cover the same evidence handling with
  controlled fixtures

#### Scenario: A test runs its own binary as a helper process

- **WHEN** a test runs its test binary again as a helper process with the
  test's environment, and the helper exits inside a test
- **THEN** the helper SHALL apply the same host Git policy as the test that
  started it
- **AND** after a complete local verification the temporary directory SHALL
  hold no directory that the verification created for its own run, although a
  tool's persistent cache MAY remain

### Requirement: Source-bound quantitative evidence

A quantitative acceptance observation MUST retain its raw counts and bind the
measured source revision, tree, toolchain and policy through the owning
verification record. Displayed percentages SHALL derive from those counts.
Native output and its invocation context MAY jointly carry this evidence;
tracked source SHALL NOT require a self-referential commit identifier or a
parallel claim-digest registry.

#### Scenario: Quantitative evidence is incomplete or inconsistent

- **WHEN** an acceptance observation omits its measured source context or raw
  counts, or its percentage disagrees with those counts
- **THEN** it SHALL NOT establish the claimed quantitative acceptance
- **AND** the native measurement owner SHALL retain the raw failure rather than
  manufacture missing facts in a second record.

### Requirement: Native client fixtures are repository-controlled

Cross-platform tests SHALL construct client executables from test-owned
fixtures rather than borrowing unrelated host toolchain executables.

#### Scenario: Windows tests an unreadable Claude executable

- **WHEN** native Windows verification exercises client-executable read failures
- **THEN** the fixture SHALL be an isolated executable controlled by the test
- **AND** the result SHALL not depend on the installed Go toolchain path or contents.

### Requirement: Native Markdown formatting preserves semantic containers

Current authored Markdown SHALL use the locked original Prettier formatter with
preserved prose wrapping and native embedded-language formatting. The existing
fix and check commands SHALL consume the same declared inventory without ambient
configuration. Other lint rules SHALL accept formatter-canonical containers;
immutable archives and native byte-exact ignores SHALL remain unchanged.

#### Scenario: Canonical lists retain their semantic separation

- **WHEN** native Prettier formats tight, loose, nested or quoted lists, including
  wrapped paragraphs, fenced examples and tables
- **THEN** the existing check SHALL accept the native output without a spacing
  postprocessor or custom paragraph parser
- **AND** native ignored examples and unrelated source SHALL retain their bytes.

#### Scenario: A real formatting journey converges once

- **WHEN** authored inputs differ from their native canonical output
- **THEN** the check SHALL refuse without mutation
- **AND** the fix SHALL produce the native output on that same inventory
- **AND** a second fix SHALL leave every byte unchanged and the check SHALL pass.
