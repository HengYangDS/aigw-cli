## MODIFIED Requirements

### Requirement: Terminal repository closure

AIGW SHALL admit a release only when canonical specifications contain no
placeholder authority, every direct repository dependency is current and
stable, transitive selection remains owned by those direct dependencies, every
package and aggregate statement coverage remain strictly greater
than 95 percent, native macOS, Linux, and Windows source verification pass, and
GitLab plus GitHub each publish and verify their own signed release.

#### Scenario: A stable direct dependency update is available

- **WHEN** the declared Go toolchain reports a newer stable direct module version
- **THEN** `go.mod` and `go.sum` SHALL be refreshed together
- **AND** the complete native source gate SHALL pass before integration

#### Scenario: Only an unneeded transitive update is reported

- **WHEN** the module query reports a newer transitive version but `go mod why`
  shows that the main module does not need that module
- **THEN** AIGW SHALL leave selection with the direct dependency owner
- **AND** SHALL NOT add an explicit pin merely to display the newest version

#### Scenario: A canonical document contains placeholder authority

- **WHEN** a specification purpose remains `TBD` or describes generation history
- **THEN** terminal closeout SHALL fail until the purpose states current product
  semantics directly

#### Scenario: Protected branches are projected

- **WHEN** the operator selects `main` for a Forge identity projection
- **THEN** every `main` and `dev` precondition SHALL pass before publication
- **AND** one atomic push SHALL advance both protected branches or neither
- **AND** only an explicit `proposal/*` selection MAY use single-branch projection
- **AND** candidate, work, or arbitrary branches SHALL be rejected.

#### Scenario: One Forge is unavailable

- **WHEN** either GitLab or GitHub cannot publish or verify its release
- **THEN** the other Forge MAY remain independently usable
- **AND** the product SHALL NOT claim dual-Forge closure

#### Scenario: Accepted product delivery completes

- **WHEN** both Forge releases and installed runtime behavior are verified
- **THEN** absorbed Work Lanes SHALL be retired through the adopted ETHOS public
  command
- **AND** no work, candidate, compatibility, or historical implementation plane
  SHALL remain as a product surface
