## MODIFIED Requirements

### Requirement: Package installation preserves ownership

Package-manager installations SHALL delegate binary replacement, rollback and removal to that manager. Installation SHALL NOT read credentials, require Proxy, or configure clients implicitly. Product removal SHALL provide an explicit path to detach enabled client projections while preserving user configuration and credentials by default.

#### Scenario: Homebrew owns the executable

- **WHEN** a user requests product-managed replacement, rollback or removal
- **THEN** AIGW SHALL identify Homebrew ownership and give the corresponding manager operation
- **AND** it SHALL NOT overwrite Homebrew-managed files or change a credential.

#### Scenario: A package is installed before clients exist

- **WHEN** the user installs AIGW without Claude, Codex, or provider credentials
- **THEN** the executable SHALL install successfully without configuration or credential prompts
- **AND** setup and client synchronization SHALL remain explicit later actions.
