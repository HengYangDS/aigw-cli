# Proposal

## Why

Hermes 0.21.6 keeps per-profile boot-maintenance records in the same
`installs/<key>/` directory as the committed dependency environment. The
accepted Hermes verification links that whole directory into its disposable
home, where Hermes treats the home as profile `default`. After a Hermes revision
change, the first verification therefore runs boot maintenance for the
disposable home and stamps the operator's own record. The operator's home then
skips its config migration and state-database guard for that revision. That
violates the accepted requirement that removing the disposable home leaves the
operator's installation state unchanged.

## What Changes

- Create the disposable verification home outside the operator's physical
  Hermes data roots. Borrow only the committed dependency entries
  (`facts.json`, `environments/`, `pm-runtime`), the original selection lock and
  the managed `tools` store. Keep boot records private to the disposable home,
  which removes them with it.
- Refuse a recorded dependency selection whose original lock is missing.
- Disable Hermes launcher publication (`cli.expose_on_path: false`) in the
  verification configuration.
- Refuse Hermes verification on Windows before starting the client: installed
  Hermes ignores that setting there, so verification could republish the
  operator's launchers. Windows builds select this refusal at build time and
  carry no borrowing or probe code they could never run.
- Remove the replaced whole-directory sharing.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: "Hermes verification reuses installed dependency
  state" gains a scenario for refusing verification when native launch effects
  cannot be isolated, and its main scenario names a disposable home outside the
  operator's native data roots.

## Impact

The Hermes adapter, its Hermes configuration owner, and their tests change. No
dependency, configuration owner, credential path, or client configuration write
is added.

Authority boundary: AIGW owns only its disposable verification home. Hermes owns
its configuration, installation state, boot records and launchers; verification
borrows dependency entries read-only and changes none of them.

Reuse stance: this ports the reviewed implementation of terminal-lane commit
`46f451c4` onto accepted `dev`. The bodies of `CreateVerificationHome` and
`borrowVerificationInstall` are copied unchanged, and that commit's runtime
Windows check becomes a build constraint, so a later merge of that lane keeps
one implementation. The requirement text keeps its current wording, which
stays accurate and under the strict 500-character limit.

Non-goals: an upstream-supported Hermes isolation mechanism, Windows and Linux
qualification, external Provider inference, the release real-client fixture's
own Hermes home, and the terminal lane's other work.
