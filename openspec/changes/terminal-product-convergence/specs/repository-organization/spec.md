## ADDED Requirements

### Requirement: Release chronology shares strict version semantics

Release headings and tags SHALL consume the same strict SemVer implementation
already used by release construction and update admission. Published headings
SHALL have strictly decreasing precedence. Exact tag and release-epoch lookup
SHALL retain build metadata, which does not alter precedence. Values outside
the supported numeric core SHALL fail rather than silently overflow.
Native source admission and release construction SHALL consume one strict
reader of the canonical `VERSION` carrier before executing downstream work.

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

## MODIFIED Requirements

### Requirement: Governed release-branch convergence

Declared candidate, accepted and release refs SHALL advance through current
transition authority and tracked branch-role policy. Normal governance and an
explicitly authorized, exact-scope maintainer recovery SHALL retain proof,
identity, compare-and-swap and post-effect observation requirements.
The adopter SHALL declare its product roles and gates without prescribing ETHOS
internal transition names or capability schema. GitLab and GitHub SHALL publish
the same accepted source independently; remote availability SHALL not be a prerequisite
for local proof or release assembly. Source integration MAY retain an active
official Change while delivery is unfinished; its task carrier SHALL remain
the sole progress authority until obligations are settled and archived.

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

### Requirement: Semantic documentation architecture

Documentation SHALL have one global entry point and semantic organization.
Official OpenSpec artifacts are the sole tracked change-intent authority; ETHOS
MAY derive transient execution inputs through its current public contract. The
adopter SHALL NOT freeze ETHOS internal fields or persist a parallel intent
carrier. Filenames MUST name their subjects. Local indexes or extra
carriers MAY exist only for semantics not representable by OpenSpec, the global
entry point, or existing authorities, and MUST declare owner, consumer,
replaced authority, and retirement.

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
