## MODIFIED Requirements

### Requirement: File persistence is owner-only and atomic

The file backend SHALL use a verified owning directory and regular Token
files, with guarded same-directory replacement, and SHALL enforce current-user
ownership, owner-only modes and link-count restrictions.

#### Scenario: Fresh file store

- **WHEN** AIGW first persists a Token in the file backend
- **THEN** AIGW SHALL require directory mode `0700` and Token file mode `0600`

#### Scenario: Unsafe storage object

- **WHEN** the file backend detects an invalid root, a nonregular Token
  file or a path-identity change
- **THEN** AIGW SHALL fail before returning or changing Token material
- **AND** AIGW SHALL also reject symbolic or multiply linked files,
  unsafe modes and foreign ownership
