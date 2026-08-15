## MODIFIED Requirements

### Requirement: Governed release-branch convergence

Local `candidate/dev`, protected `dev`, and protected `main` SHALL advance
through the repository's public governance lifecycle. The tracked branch-role
policy SHALL declare one `accepted-to-release` transition from `accepted_root`
to `release_root`, require `proof:execution`, and grant only the
`repository.release` capability. GitLab and GitHub SHALL publish the same
accepted source independently; remote availability SHALL not be a prerequisite
for local proof or release assembly.

#### Scenario: Accepted content is ready for release

- **WHEN** exact-head proof has admitted the candidate and accepted `dev` is current
- **THEN** the declared governed release transition advances `main` from that accepted content
- **AND** local readiness remains distinct from remote publication.

#### Scenario: Direct branch mutation is attempted

- **WHEN** an actor attempts to bypass the governed lifecycle for `candidate/dev`, `dev`, or `main`
- **THEN** repository admission blocks the mutation
- **AND** reports the required public governance operation.

#### Scenario: One Forge is unavailable

- **WHEN** one publication plane cannot be reached
- **THEN** the other may publish and verify the same signed revision independently
- **AND** local accepted state remains valid without either remote.
