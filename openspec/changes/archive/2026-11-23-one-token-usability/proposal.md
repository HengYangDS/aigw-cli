## Why

The owner chose on 2026-11-22 at 19:43 to land `client-chosen-model` with two root causes open and to fix each in its
own Change from `dev`, with a fresh review, releasing nothing until both land. This is the second: when a credential
store cannot tell whether a provider's Token is there (a locked keychain, a Token file that the file store refuses),
synchronization answers "the client waits" (`tokenUsable`), while `SecretAvailable`, which `ConvergeBinding` and
`hermesDesired` use, returns the store's error and the command fails. The twelfth review of `client-chosen-model`
found the results: another provider's refused Token file leaves Hermes's record behind its file or refuses every
Hermes command (its finding 2), and `aigw sync` and its preview are refused when the selected provider's Token cannot
be observed (its finding 3). Its finding 5, an edit that drops a file being refused without the Token and succeeding
with it, follows from the same judgement; the owner was told on 2026-11-23 that it would be fixed here unless he said
otherwise.

## What Changes

- One judgement of a provider's Token with three answers: usable, absent, or not observable. Every command decides
  by it, and a Token that cannot be observed counts as an absent one for each client that uses it: that client
  waits, the command goes on for the others, and the result names the store's failure and its fix.
- `ConvergeBinding` and Hermes's plans no longer fail a command on a store that cannot tell; Hermes leaves out the
  models of a provider whose Token cannot be observed, as it leaves out those of a provider without a Token.
- **Changed behaviour:** `aigw client edit <client> --target` that drops one of the client's files is no longer refused
  when the provider has no Token: AIGW withdraws its settings and record from each dropped file, which needs no Token,
  and the files that the client keeps wait for the Token as before.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `client-chosen-model`: the requirement that a model chosen in Codex or Hermes is that client's selection changes one
  scenario's outcome (a dropping edit without a Token); a requirement is added for a Token that its store cannot
  observe.

## Impact

`internal/secret` (the judgement's one owner), `internal/adapter` (`ConvergeBinding`, `SecretAvailable`),
`internal/adapter/hermes`, `internal/app/synchronization`, `internal/cli/client`, their tests, the CHANGELOG and the
command documentation of `aigw client edit`. No configuration or report schema changes.
