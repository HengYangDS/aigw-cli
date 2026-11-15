## Why

Seven commands each open a scope in which a provider's Token is observed once; the other commands open none
and ask the credential worker again for every step. The third review of `keychain-predecessor-retirement` ran
`aigw provider edit` and saw the worker asked twice whether one slot exists and twice whether it is empty,
once for the preview and once for the commit. A rule that seven sites repeat is missing at the eighth, and each
extra call is one more native operation under its bound.

## What Changes

- One availability scope for each command, opened where every command starts; the seven scopes that commands
  open themselves go. A provider's Token and its balance credentials share that scope.
- A command asks the credential backend for each fact of a credential's availability once, and again after the
  command itself stores or removes that credential, also when it removes one to undo its own change.
- Fixed with it: `aigw use` removed a Token that it had stored when its selection then failed, and said so
  only with a credential backend that `AIGW_SECRET_BACKEND` names. With the backend that AIGW selects itself,
  the undo ran outside the scope, which kept the Token as present. Releases 0.7.0 to 0.8.0 have the defect.
- Reading a credential's value is unchanged and is never served from a scope.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secret-storage`: "Credential availability is observable without disclosure" gains the rule that one command
  observes a credential once, and observes it again after its own change or its own undo.

## Impact

- `internal/cli` (the place where a command's context is built, and seven commands), `internal/secret` (the
  undo of a stored Token, and the balance credentials' view), acceptance tests that count the worker's
  operations.
- One message changes: the fixed case of `aigw use` now states that the Token was not kept. No JSON document
  gains or loses a field, and no exit status changes.
- Out of scope: the 5 s bound of a native credential operation, which is reviewed on 2026-11-20.
