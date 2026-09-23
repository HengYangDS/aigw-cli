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

Candidate, accepted, and release refs SHALL advance through current authority
and tracked branch roles. Normal and exact-scope maintainer paths SHALL preserve
proof, identity, compare-and-swap, and post-effect observation. The adopter
SHALL declare product roles without encoding ETHOS internals. Each Forge SHALL
independently publish the same accepted source; local work SHALL require neither
remote. OpenSpec SHALL remain progress authority until delivery is complete and
archived.

#### Scenario: Accepted content is ready for release

- **WHEN** exact-head proof has admitted the candidate and accepted `dev` is current
- **THEN** the declared governed release transition advances `main` from that accepted content
- **AND** local readiness remains distinct from remote publication.

#### Scenario: Direct branch mutation is attempted

- **WHEN** an actor attempts to move a declared ref without current exact-scope
  authority or matching observed source and destination objects
- **THEN** admission blocks the mutation
- **AND** reports the missing authority or changed precondition
- **AND** a governor defect requires an explicit bounded maintainer recovery,
  not a permanent hook bypass or a replacement lifecycle engine.

#### Scenario: One Forge is unavailable

- **WHEN** one publication plane cannot be reached
- **THEN** the other may publish and verify the same signed revision independently
- **AND** local accepted state remains valid without either remote.

### Requirement: Portable repository quality surface

Repository configuration, tools, source, tests, documentation, and release
assets SHALL be organized by semantic owner. Root-level sprawl, forwarding
scripts, concatenated package names, cross-package private calls, and duplicated
policy SHALL not create parallel authority.

#### Scenario: A contributor follows one behavior

- **WHEN** a contributor traces a public command or projection
- **THEN** implementation, tests, specification, and documentation identify one owner
- **AND** no compatibility facade or duplicate configuration must be consulted.

#### Scenario: Contributor uses another host

- **WHEN** the repository is verified on macOS, Linux, or Windows
- **THEN** repository-owned Go commands and declarative configuration provide the same contract
- **AND** shell-specific behavior is not required for product or CI correctness.

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
