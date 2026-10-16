## MODIFIED Requirements

### Requirement: Governed release-branch convergence

Candidate, accepted, and release refs SHALL advance through current authority and the shared governed branch roles. Normal and exact-scope maintainer paths SHALL preserve proof, identity, compare-and-swap, and post-effect observation. The adopter SHALL consume the shared roles without a competing configurable role policy or an interpretation of ETHOS internals. Each Forge SHALL independently publish the same accepted source; local work SHALL require neither remote. OpenSpec SHALL remain progress authority until delivery is complete and archived.

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
- **AND** the current Forge output retains its event, branch, identity and admission semantics.

### Requirement: Portable repository quality surface

Repository configuration, tools, source, tests, documentation, and release assets SHALL be organized by semantic owner. Native tool settings SHALL have one consuming tool carrier, and repository-specific parameters SHALL retain one owning policy. Every source-quality and release consumer SHALL read its declared carrier explicitly and refuse an absent or malformed required configuration. Root-level sprawl, forwarding scripts, concatenated package names, cross-package private calls, and duplicated policy SHALL not create parallel authority. Unrelated external consumers SHALL own their adaptation and qualification outside this product's implementation and quality dependency graph.

#### Scenario: A contributor follows one behavior

- **WHEN** a contributor traces a public command or projection
- **THEN** implementation, tests, specification, and documentation identify one owner
- **AND** no compatibility facade or duplicate configuration must be consulted.

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
