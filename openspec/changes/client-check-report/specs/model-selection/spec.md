## MODIFIED Requirements

### Requirement: A client-depth check runs the client privately

A `client`-depth check SHALL run the real client privately, with a private
client configuration that reaches the operator's user-owned files without
copying or owning them.

#### Scenario: A client session leaves operator extensions out

- **WHEN** a `client`-depth check runs a real client
- **THEN** the client SHALL run in a private home or its native isolation mode
  without starting the operator's MCP servers, plugins, hooks, notifications or
  memory providers
- **AND** Codex's own memory work SHALL be off in that home
- **AND** an operator extension that cannot start or authorize there SHALL NOT
  decide the result
- **AND** for Hermes the private home SHALL name no MCP servers or shell hooks
  and carry no hook consent, and the operator's configuration SHALL stay
  unchanged.

#### Scenario: The operator's Codex catalog is not an absolute path

- **WHEN** the operator's own `model_catalog_json` is relative to the Codex
  configuration that names it, or on Windows rooted without a drive, such as
  `\catalogs\models.json`
- **THEN** the private client configuration SHALL name that file by its
  absolute path, on the drive of that configuration for a rooted path
- **AND** a reference that names a drive without a root, such as
  `D:models.json`, the catalog and the operator's configuration SHALL stay
  unchanged, and AIGW SHALL NOT take ownership of the catalog.

#### Scenario: A client reports a warning or an error

- **WHEN** the session of Codex or Claude Code succeeds but the client reports
  a warning or an error of its own
- **THEN** the check SHALL stop at the `client` depth with AIGW's own message,
  which SHALL carry none of the client's sentences
- **AND** for a configuration key that the client calls deprecated, written as
  a table in brackets and a name, the message SHALL name the key and the Fix
  SHALL be to remove it, unless the client also reports that it lacks the
  model's metadata, which the message then names instead
- **AND** with `--verbose` the text and the JSON output SHALL show the client's
  own lines: at most four, each of at most 300 characters, without the
  provider's Token and the known credential forms, in the text after AIGW's
  own lines
- **AND** without `--verbose` neither output SHALL carry them
- **AND** the Fix SHALL be one sentence and SHALL NOT be `aigw check`.

#### Scenario: AIGW does not hold the session's credential

- **WHEN** such a client authenticates itself, or reads its Token through an
  external credential helper
- **THEN** neither output SHALL show the client's lines, with or without
  `--verbose`
- **AND** when AIGW names nothing to change, the Fix SHALL be to run the client
  itself with the same model.
