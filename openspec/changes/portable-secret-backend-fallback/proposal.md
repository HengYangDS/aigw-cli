# Change: Provide a portable secret-backend fallback

## Why

A native credential service is not guaranteed to be usable in a headless,
locked-down, or newly provisioned user session. Windows previously bypassed
the availability probe and therefore could make first setup depend on a
working Credential Manager session. That contradicted the product requirement
that one configured Provider Account is enough to become usable.

## What Changes

- Apply the same automatic native-service probe on macOS, Linux, and Windows.
- Keep one persisted backend authority; never search multiple stores.
- Add a Windows fallback whose at-rest bytes are protected by current-user
  DPAPI rather than stored as plaintext.
- Keep explicit `keyring` fail-closed and `env` read-only.

## Boundary

AIGW still owns only local Account Token storage and client projections. This
change neither installs a proxy nor changes external services, client private
state, or ETHOS lifecycle semantics.
