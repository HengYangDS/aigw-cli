## MODIFIED Requirements

### Requirement: Portable source

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, signing key, fingerprint,
signing program, trust anchor, foreign-application private path, or external
service lifecycle. CI SHALL provide trust material only from protected context.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, host, or
  Forge
- **THEN** build, verification, setup, sync, and uninstall SHALL not require
  the original contributor's machine, account, key, IDE, or workstation state

#### Scenario: An operator uses the installed command

- **WHEN** another team installs AIGW on a supported host
- **THEN** the installed executable SHALL provide the user command surface
- **AND** repository tools SHALL remain developer-only entrypoints
- **AND** documented configuration, credentials and discovery inputs SHALL not
  require an author-specific path, identity, key, service or foreign product.

#### Scenario: An operator selects environment credentials

- **WHEN** `AIGW_SECRET_BACKEND=env` is selected
- **THEN** the documented provider variable mapping SHALL supply credentials
  without persistence
- **AND** the user guide SHALL explain process inheritance and direct installed
  command invocation without exposing real credentials.
