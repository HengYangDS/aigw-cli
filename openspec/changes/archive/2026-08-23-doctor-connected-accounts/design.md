## Decision

Use the existing Route resolution model as the sole definition of an Account
that is currently required. For every admitted client, resolve its active
runtime and collect the selected Account ID. Diagnose a missing Token once per
selected Account in stable lexical order.

Do not add a connected-account registry, flag, cache, or persisted state.
Connection remains observable from the existing Route and credential
authorities. An Account retained only as catalogue capability is not a current
runtime dependency and therefore cannot make the installation unhealthy.

If multiple client Routes select the same Account, emit one credential check.
If a selected Account lacks its Token, retain the existing actionable
`aigw rotate <account>` repair.
