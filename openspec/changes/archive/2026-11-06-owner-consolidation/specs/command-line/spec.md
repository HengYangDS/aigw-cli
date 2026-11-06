## MODIFIED Requirements

### Requirement: Every write can be previewed

Every command that writes configuration, client files or the program SHALL
accept `--dry-run`. A preview SHALL report its plan with `dry_run: true`, read
only what the plan needs, send no authenticated request, download nothing, run
no other program, and leave no file, credential or Keychain change behind.

#### Scenario: Setup is previewed

- **WHEN** `aigw setup --dry-run` runs with an Account, a Route and a client
- **THEN** it SHALL report the Account, Route and Client Binding that it would
  create
- **AND** it SHALL NOT read, check or store a Token
- **AND** the configuration, the credential store and client files SHALL be
  unchanged.

#### Scenario: Removing a Route is previewed

- **WHEN** `aigw route remove <route> --dry-run` runs
- **THEN** it SHALL report the Client Bindings and recommendations that the
  removal affects
- **AND** the configuration SHALL be unchanged.

#### Scenario: A preview refuses what its write refuses

- **WHEN** `aigw sync --dry-run` or `aigw repair --dry-run` meets a
  configuration transition that `aigw sync` or `aigw repair` refuses
- **THEN** the preview SHALL refuse it with the error the write gives
- **AND** it SHALL report no plan.
