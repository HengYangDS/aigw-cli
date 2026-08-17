## ADDED Requirements

### Requirement: Forge capability projection

The repository SHALL expose one product evidence graph and one deterministic CI
topology authority. Product evidence requirements SHALL be modeled separately
from each Forge's admitted executor capacity. A Forge projection SHALL contain
only native jobs that Forge can execute, while the aggregate product evidence
set SHALL retain every supported native platform. A missing executor on one
Forge SHALL NOT create an optional, indefinitely pending, or `allow_failure`
substitute and SHALL NOT weaken product support.

#### Scenario: one Forge lacks a Windows executor

- **WHEN** another independent publication plane supplies admitted native Windows evidence
- **THEN** the Forge without Windows capacity SHALL omit its Windows job
- **AND** the product evidence model SHALL continue to require Windows
- **AND** cross-compilation SHALL NOT be reported as native evidence.

#### Scenario: Windows capacity is admitted later

- **WHEN** GitLab gains a qualified Windows executor
- **THEN** one capability declaration SHALL restore the generated native Windows job
- **AND** no parallel workflow or compatibility switch SHALL be introduced.
