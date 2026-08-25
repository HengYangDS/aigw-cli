# Design: Portable single-backend Token storage

## Authority

One persisted selector owns which writable backend contains Account Tokens.
Automatic selection probes the native keyring once and pins either `keyring`
or `file`. Subsequent operations use only the pinned backend.

## Platform protection

- macOS and Linux use the existing owner-only directory and regular-file
  invariants.
- Windows stores only DPAPI ciphertext bound to the current user. Paths are
  resolved through an `os.Root`, and writes replace a temporary sibling file.
- Automation may explicitly select `env`; it remains process-scoped and
  read-only.

This avoids a parallel compatibility reader while ensuring that lack of a
desktop keyring does not make setup unusable.

## Verification

Unit tests cover selection semantics and Unix persistence. Windows compilation
proves the platform implementation is complete; native Windows CI must execute
the DPAPI round trip and the setup journey before release admission.
