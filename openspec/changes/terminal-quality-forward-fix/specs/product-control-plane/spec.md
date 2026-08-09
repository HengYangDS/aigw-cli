## MODIFIED Requirements

### Requirement: Local-first independent publication topology

AIGW SHALL declare one executable local verification command, one executable
local installation command, and separate GitLab and GitHub remotes and CI
surfaces. Publication admission MUST reject an incomplete declaration and MUST
NOT require either Forge to operate the other. The specification carrying this
contract SHALL use the canonical text form enforced by repository policy.

#### Scenario: One Forge is unavailable

- **WHEN** local verification and one declared Forge remain available
- **THEN** local acceptance and that Forge's publication path SHALL remain
  independently inspectable without mutating or querying the unavailable Forge

#### Scenario: The canonical specification is verified

- **WHEN** the repository architecture gate reads the product-control-plane spec
- **THEN** it SHALL find exactly one terminal newline
- **AND** the local-first independent publication requirement SHALL remain unchanged
