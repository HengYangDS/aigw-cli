# Proposal

## Why

AIGW has qualified portable release candidates, but its release validator rejects every non-prerelease version and its distribution contract excludes public macOS notarization. The owner now authorizes formal distribution with the established Developer ID identity and a maintained Homebrew tap.

## What Changes

- Separate semantic version validity, product acceptance, and platform distribution trust.
- Extend the existing release flow to sign and notarize macOS distribution artifacts, then checksum and publish those exact bytes to both selected peers.
- Generate Homebrew packaging from the accepted release inventory; preserve one installation owner and keep setup explicit.
- Preserve reproducible credential-free construction as a distinct verification boundary from timestamped distribution signing.
- **BREAKING**: replace the unconditional prerelease-only policy; stable admission requires concrete product evidence, not an impossible version predicate or an invented Windows certificate prerequisite.

## Capabilities

### New Capabilities

- `release-distribution`: immutable public release delivery and package-manager ownership.

### Modified Capabilities

- `product-quality`: extend complete delivery evidence to public macOS signing and notarization while retaining all existing native, credential, and peer-parity obligations.

## Impact

Existing release readiness, construction, publication, upgrade ownership, GoReleaser configuration, CUE-owned CI, and operational documentation are affected. Reuse Apple native tooling, GoReleaser, and Homebrew rather than add a signing framework. Private keys remain operator-controlled. ETHOS owns generic lifecycle admission. Proxy remains optional and unchanged; client credentials and running sessions are outside this change's mutation scope.
