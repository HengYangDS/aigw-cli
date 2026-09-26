# Spec Delta

## MODIFIED Requirements

### Requirement: Upgrade evidence preserves credential continuity

Upgrade acceptance SHALL retain each enabled client's command, arguments,
environment, native item, reader, and authorization identity. Before a package
manager unlinks its CLI path, every retained caller SHALL use a qualified
reader independent of that path or the cutover SHALL stop. Original invocations
SHALL return the same Token before, during, and after replacement without
reload; a fresh client or helper SHALL NOT substitute for existing-caller
continuity. A successor reader SHALL be qualified before any client projection
selects it, while retained callers keep their original reader.

#### Scenario: A proposed adapter survives only CLI-only updates

- **WHEN** unchanged adapter bytes retain item access while the CLI changes
- **THEN** actual adapter replacement, rollback and caller authorization SHALL
  remain unproved until their own retained-item journeys pass
- **AND** preserving a vulnerable old reader SHALL NOT satisfy safe updates.

#### Scenario: A client retains its credential command across an update

- **GIVEN** a client has already loaded a valid credential command and environment
- **WHEN** the installed AIGW program is replaced without changing its route
- **THEN** that retained invocation SHALL return the original authorized Token
  before synchronization or client restart
- **AND** updated projections or success from a new helper SHALL NOT substitute
  for that invocation
- **AND** native credential denial SHALL block production acceptance rather than
  trigger an alternate reader, ACL change or repeated authorization attempt.

#### Scenario: The package manager temporarily removes the CLI path

- **GIVEN** an update has been admitted for clients retaining original
  credential commands
- **WHEN** a package manager removes or replaces the CLI executable path
- **THEN** those original invocations before, during, and after that interval
  SHALL return their authorized Token without a missing-executable gap
- **AND** no client reload, Token migration, ACL change, or credential prompt
  SHALL be needed
- **AND** failure SHALL preserve or restore a working original invocation.

#### Scenario: A legacy client still calls the manager-owned CLI link

- **WHEN** a retained caller still uses the executable path a package manager
  will unlink
- **THEN** production cutover SHALL stop before that unlink
- **AND** rewriting a configuration file or testing a fresh client SHALL NOT
  claim the cached caller was migrated.

#### Scenario: The last default Token consumer is withdrawn

- **WHEN** a successful projection disables or replaces the last default
  Account-Token Client Binding
- **THEN** AIGW SHALL remove only intact, owned credential executables with no
  remaining configured or retained caller after projection completion
- **AND** another default consumer or incomplete projection rollback SHALL
  preserve the exact executable it may still invoke
- **AND** an enabled explicit credential command resolving to an owned reader
  SHALL preserve it without changing that command's owner
- **AND** a failed removal SHALL report committed client state with incomplete
  cleanup, not a rolled-back transition or silent success.

#### Scenario: A qualified reader successor is installed

- **GIVEN** a client still invokes an older AIGW-owned reader
- **WHEN** a signed successor is prepared for an installed-product update
- **THEN** AIGW SHALL verify the successor's exact bytes, native credential
  access, selected projection and rollback before switching any client
- **AND** SHALL NOT overwrite an executable still used by a retained caller
- **AND** failure SHALL preserve the older command and Token behavior.

#### Scenario: A retired owned credential executable has no consumer

- **WHEN** no configured client, retained caller, rollback target or explicit
  external binding can invoke an owned older executable
- **THEN** AIGW SHALL remove only that exact executable and its owned identity
  evidence
- **AND** SHALL preserve unknown or foreign content rather than deleting by
  name prefix or age.
