# Hermetic Forge test Git configuration

## Why

Temporary Git repositories created by Forge tests inherited the host's global
commit-signing and hook configuration. On a macOS release runner this made an
ordinary fixture commit wait on the workstation Keychain until the package
test timeout expired, while clean hosted runners passed the same product tree.

## What changes

- give every unsigned replay fixture explicit local Git signing and hook
  policy;
- preserve fixture-owned signing only where a test verifies signatures;
- publish a forward release candidate after native macOS acceptance passes.

## Non-goals

- no product runtime, Forge protocol, signing policy, or release-topology
  change;
- no timeout increase, retry loop, platform exception, or compatibility path.
