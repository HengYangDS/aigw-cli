## Decision

Credential remediation follows the selected secret store's mutability:

- a writable store directs the operator to `aigw rotate <account>`;
- the environment store names the exact variable returned by the existing
  `secrets.EnvironmentKey` authority and never suggests a write command.

The existing credential domain owns this recovery policy because it already
owns provider-neutral Token validation and can consume the secret store's
capability without coupling peer CLI packages. The secret package continues to
own backend capability and environment-key derivation; command packages only
render the resulting instruction.

`rotate` checks store mutability after resolving the Account and before reading
input, validating credentials, or attempting persistence. Setup, import, route
selection, readiness, verification, and adapter paths consume the same helper so
future commands cannot accidentally revive an impossible recovery instruction.

No compatibility alias, new state carrier, or additional command is introduced.
