# Proposal

## Why

Pull request HengYangDS/aigw-cli#165 ran native Windows acceptance on accepted
`dev` `a278c443` for the first time.
`TestExternalCredentialFailuresKeepUnknownSecretsOutOfDiagnostics/hermes`
failed. On Windows, Hermes verification refuses before starting the client, as
the accepted requirement specifies, but the test still expected a categorized
credential failure after two client calls. The archived Change
`2026-10-12-hermes-verification-boot-isolation` updated the adapter tests for
that refusal and missed this registry-level test, so GitHub's required checks
cannot accept `a278c443` on `dev`.

## What Changes

- On Windows, the credential-policy test requires Hermes verification to refuse
  before starting the client, start no client process, and keep the external
  credential canary out of the error.
- The refusal scenario states that the refusal discloses no credential
  diagnostics.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: the "Native launch effects cannot be isolated"
  scenario of "Hermes verification reuses installed dependency state" also
  requires that the refusal discloses no credential diagnostics.

## Impact

One test in `internal/client` and one spec scenario change. Product code is
unchanged.

Authority boundary: unchanged from the archived Change. AIGW owns only its
disposable verification home.

Reuse stance: the test reuses the refusal expectation the adapter tests already
assert, written inline because this test lives in the external `client_test`
package.

Non-goals: the GitLab tool-mirror packages that the protected pipelines cannot
download, the terminal lane's identical test, and Windows qualification beyond
the hosted gate.
