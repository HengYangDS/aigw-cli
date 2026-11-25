## REMOVED Requirements

### Requirement: Windows path derivation preserves the selected namespace

**Reason**: AIGW no longer supports Windows, so no derived path starts from a UNC, extended-length or drive namespace.

**Migration**: None; macOS and Linux derive every path from its POSIX base as before.
