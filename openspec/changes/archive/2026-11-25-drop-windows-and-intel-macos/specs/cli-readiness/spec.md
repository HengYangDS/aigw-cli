## REMOVED Requirements

### Requirement: Local checks know a target on another volume is outside home

**Reason**: On macOS and Linux every absolute path shares one root, so only Windows had a target on another volume.

**Migration**: "Local checks name the native file they inspect" keeps labelling a target outside the home directory.
