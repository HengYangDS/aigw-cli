# Proposal

## Why

The second hosted Native Windows acceptance of accepted `dev` `ef404988`
(GitHub Actions run `38245567639`, job `114798438309`) failed
`TestNativeVerificationResources` in all five modes. That native acceptance
qualifies that verification reclaims the client process tree and disposable
state it owns on success, failure, early parent exit, interruption and
deadline. It drives verification only through Hermes, which now refuses on
Windows before starting a client, so Windows lost its only native qualification
of verification process-tree reclamation.

## What Changes

- Run the native verification resource cases through Codex on every host, and
  through Hermes as well on hosts where Hermes verification starts a client.
- Let the shared client fixture hold the owned parent and child processes in
  Codex's first verification turn, then answer the turn as the existing Codex
  fixture does.
- Record each case's disposable home from the client's own variable
  (`HERMES_HOME` or `CODEX_HOME`).
- State in the Codex verification requirement that verification reclaims its
  owned resources.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: "Real Codex client route verification" gains a
  scenario requiring Codex verification to reclaim its owned process tree and
  verification workspace in every outcome.

## Impact

Test code in `tools/release` and one spec scenario change. Product code is
unchanged. Native acceptance on macOS and Linux runs one more client, about a
minute longer, mostly for the deadline case.

Authority boundary: AIGW owns only the verification processes and workspace it
creates. The test's unrelated sentinel process and protected files stay
untouched.

Reuse stance: the Codex resource case reuses the existing resource parent,
child and sentinel roles and the existing Codex verification fixture.

Non-goals: changing verification behavior, the Codex terminal lane's identical
test, and the GitLab publication that waits for the network.
