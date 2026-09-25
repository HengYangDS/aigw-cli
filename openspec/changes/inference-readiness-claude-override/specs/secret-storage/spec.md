## MODIFIED Requirements

### Requirement: Upgrade evidence preserves credential continuity

Upgrade acceptance SHALL retain each enabled client's original credential
command, arguments, environment, native item, reader implementation, and
authorization identity. Original invocations SHALL return the same Token
before, during, and after program replacement, including a package-manager
unlink interval, without synchronization or client reload. A fresh client or
helper SHALL NOT substitute for existing-caller continuity.

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

- **GIVEN** enabled clients retain their original credential commands
- **WHEN** a package manager removes or replaces the CLI executable path
- **THEN** every invocation issued before, during, and after that interval
  SHALL return its original authorized Token without a missing-executable gap
- **AND** no client reload, Token migration, ACL change, or credential prompt
  SHALL be needed
- **AND** failure SHALL preserve or restore a working original invocation.
