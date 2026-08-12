## MODIFIED Requirements

### Requirement: Local-first independent publication topology

AIGW SHALL declare one repository-owned local verification command, one
repository-owned local installation command, and independent GitLab and GitHub
publication peers. Each peer SHALL own its remote and CI surface. Publication
admission MUST reject an incomplete declaration and MUST NOT make either Forge
depend on the other.

#### Scenario: One Forge is unavailable

- **WHEN** local verification and one declared Forge remain available
- **THEN** local acceptance and the available Forge publication path remain
  independently operable without querying or mutating the unavailable Forge

#### Scenario: The canonical specification is verified

- **WHEN** the repository architecture gate reads the product-control-plane
  specification
- **THEN** it SHALL find exactly one terminal newline
- **AND** the local-first independent publication requirement SHALL remain
  unchanged
