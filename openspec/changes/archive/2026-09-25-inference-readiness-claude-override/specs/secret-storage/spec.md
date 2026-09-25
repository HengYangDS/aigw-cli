## MODIFIED Requirements

### Requirement: Upgrade evidence preserves credential continuity

Upgrade acceptance SHALL retain each enabled client's command, arguments,
environment, native item, reader, and authorization identity. Before a package
manager unlinks its CLI path, every retained caller SHALL use an external
stable entrypoint or the cutover SHALL stop. Original invocations SHALL return
the same Token before, during, and after replacement without reload; a fresh
client or helper SHALL NOT substitute for existing-caller continuity.

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

- **GIVEN** enabled clients retain original commands to an admitted stable
  credential entrypoint
- **WHEN** a package manager removes or replaces the CLI executable path
- **THEN** every invocation issued before, during, and after that interval
  SHALL return its original authorized Token without a missing-executable gap
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
- **THEN** AIGW SHALL remove only its intact credential entrypoint and receipt
  after projection completion
- **AND** another default consumer or incomplete projection rollback SHALL
  preserve the pair
- **AND** an enabled explicit credential command resolving to the owned
  executable SHALL preserve it without changing that command's owner
- **AND** a failed removal SHALL report committed client state with incomplete
  cleanup, not a rolled-back transition or silent success.
