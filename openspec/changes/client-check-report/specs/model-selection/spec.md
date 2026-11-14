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
- **AND** Codex's own memory work SHALL be off in that home, so that the
  session sends requests for the checked model only
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

- **WHEN** a client's session succeeds but the client reports a warning or an
  error of its own
- **THEN** the check SHALL stop at the `client` depth with AIGW's own message,
  which SHALL carry none of the client's sentences and SHALL name a
  configuration key that the client calls deprecated
- **AND** with `--verbose` the text and the JSON output SHALL show the client's
  own lines: at most four, each of at most 300 characters, with credentials
  redacted and the home directory written as `~`
- **AND** without `--verbose` neither output SHALL carry them, and the fix
  SHALL be the same check with `--verbose`.
