## MODIFIED Requirements

### Requirement: Semantic documentation architecture

Repository documentation SHALL use one global entry point and SHALL organize
content by semantic information domain. Official OpenSpec artifacts SHALL be
the sole tracked authority for product change intent. ETHOS SHALL derive a
transient Commitment containing only `schema_version`, `id`, and `acceptance`
from the selected OpenSpec projection when governance evaluation requires it.
Document filenames SHALL identify their subject. Container-named local indexes
and additional tracked carriers SHALL exist only when they own semantics that
official OpenSpec artifacts, the global entry point, and existing authorities
cannot represent, and SHALL name their owner, current consumer, replaced
authority, and retirement condition.

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
