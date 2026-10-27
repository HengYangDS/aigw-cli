## ADDED Requirements

### Requirement: Host-specific tooling builds only for its hosts

Repository tooling SHALL place behavior that only one family of hosts runs, such
as a POSIX process inventory, in files whose build constraints select those
hosts, with an explicit counterpart for the other hosts. Each native job's
statement coverage SHALL count only code that its host can run.

#### Scenario: Native client retirement checks for processes

- **WHEN** native client retirement checks whether a process still uses the
  supply
- **THEN** its `ps` inventory SHALL build only for Linux and macOS
- **AND** Windows SHALL build its own counterpart, which makes no inventory
- **AND** the Windows native job's coverage SHALL not count the POSIX
  inventory.
