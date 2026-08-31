## Why

Manifest setup already supports progressive Account connection and deferred
client installation, but one result contradicts that model: a connected Account
with no installed client names `aigw status` even though only `aigw sync` can
discover and activate a client installed later.

## What Changes

- Make the existing setup result name `aigw sync` for deferred client
  activation.
- Make `aigw use <profile>` converge the selected client's available adapter
  and projection instead of reporting synchronization after changing only the
  Route.
- Add acceptance coverage for the connected-Account, absent-client journey.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: make the machine-readable continuation exact
  when an Account is connected before any client is installed.
- `profile-client-selection`: require `use` to converge the selected client's
  available adapter and projection as one transaction.

## Impact

Manifest-setup continuation selection, Route selection, the existing
synchronization owner, and acceptance tests change. No command, configuration
field, dependency, compatibility path, provider priority, or external service
coupling is added.
