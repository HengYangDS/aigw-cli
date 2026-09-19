## ADDED Requirements

### Requirement: Stable identity and platform trust are distinct

AIGW SHALL validate strict semantic versions independently of platform signing. A stable release SHALL require actual product acceptance and each declared channel's trust evidence. macOS public distribution SHALL carry verified Developer ID signing and accepted Apple notarization. Windows Authenticode status SHALL be disclosed rather than inferred from a version suffix.

#### Scenario: A version parses but distribution evidence is missing

- **WHEN** a valid stable version lacks required product or channel evidence
- **THEN** distribution SHALL stop with the specific missing evidence
- **AND** version parsing alone SHALL NOT claim release readiness.

### Requirement: Final distribution bytes have one authority

Signing and notarization SHALL precede final distribution inventory and checksums. Every selected Forge and package-manager projection SHALL identify the same accepted release bytes. Reproducible construction SHALL remain a separate claim from external trusted timestamp and notarization results.

#### Scenario: A published release already exists

- **WHEN** a newly signed artifact differs from a published artifact
- **THEN** it SHALL require a new release identity rather than replacing the published bytes.

### Requirement: Package installation preserves ownership

Package-manager installations SHALL delegate binary replacement and removal to that manager. Installation SHALL NOT read credentials, require Proxy, or configure clients implicitly. Product removal SHALL provide an explicit path to detach enabled client projections while preserving user configuration and credentials by default.

#### Scenario: Homebrew owns the executable

- **WHEN** a user requests product-managed replacement or removal
- **THEN** AIGW SHALL identify Homebrew ownership and give the corresponding manager operation
- **AND** it SHALL NOT overwrite Homebrew-managed files.

#### Scenario: A package is installed before clients exist

- **WHEN** the user installs AIGW without Claude, Codex, or provider credentials
- **THEN** the executable SHALL install successfully without configuration or credential prompts
- **AND** setup and client synchronization SHALL remain explicit later actions.
