## ADDED Requirements

### Requirement: Native released-artifact lifecycle acceptance

AIGW SHALL prove the public installed-program lifecycle on macOS, Linux, and
Windows using portable archives and checksum manifests with the same shape as
published release assets. Each native journey SHALL install an older program,
update to a newer program, roll back to the immediate predecessor, recover
forward to the newer program, and uninstall. The journey SHALL preserve user
configuration and credentials according to the existing retention contract and
SHALL leave no installed executable, rollback copy, staging file, or other
owned lifecycle residue after uninstall.

#### Scenario: Released program completes the reversible lifecycle

- **WHEN** native acceptance runs on a supported operating system
- **THEN** the installed older program updates from a verified newer portable archive
- **AND** the installed program reports the newer version
- **AND** rollback restores the older version
- **AND** a second verified update restores the newer version
- **AND** uninstall removes the executable, its single rollback copy, and all staging residue
- **AND** retained configuration and credentials remain available as declared.

#### Scenario: Lifecycle evidence uses the product command plane

- **WHEN** native acceptance exercises update, rollback, forward recovery, or uninstall
- **THEN** it SHALL invoke the public AIGW command for that transition
- **AND** SHALL NOT substitute a package-level helper, platform-specific script state machine, Forge API, or external Proxy lifecycle.
