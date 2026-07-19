# AIGW Authentication Stability Design

Status: approved for implementation on 2026-07-19.

## Goal

Prevent a single transient HTTP 401 from being presented as a confirmed invalid token while preserving AIGW's configuration-control-plane boundary and keeping token changes explicitly human-authorized.

## Existing authority that remains unchanged

- Standalone Codex is AIGW-owned.
- PyCharm, JetBrains Air, and Junie CLI are JetBrains AI-owned.
- `aigw route doctor --json` remains the complete local, secret-free ownership report.
- `aigw route attest air --json` remains the bounded Air runtime-forwarding attestation.
- AIGW does not execute an IDE or Junie, inspect session databases, bypass the configured loopback endpoint, or infer billing/login from local ownership evidence.

## Stable authentication state machine

`diagnostics.Probe` remains one HTTP observation. A new `diagnostics.ProbeStable` performs:

1. One initial observation.
2. If it is not `invalid_token`, return it unchanged.
3. If it is `invalid_token`, perform three recovery observations against the same configured endpoint and the same in-memory token, with context-aware delays of 250ms, 500ms, and 1000ms.
4. Three healthy recovery observations produce `healthy`, `attempts=4`, and `recovered_transient=true`.
5. Three additional `invalid_token` observations produce persistent `invalid_token`; only this result may recommend manual `aigw rotate`.
6. Any mixed sequence produces `authentication_unstable`, `retryable=true`, no rotate recommendation, and no mutation.

Each observation receives a five-second child context. Cancellation returns promptly. No response body, token, or request body is retained.

## CLI behavior

`aigw check` uses `ProbeStable`.

- Recovered transient: success, with an informational line stating that authentication recovered after a transient response.
- Persistent invalid token: existing action-required card and manual rotate guidance.
- Authentication unstable: action-required card recommending a later retry, without rotate guidance.

No command automatically changes a token, endpoint, proxy, client, or route.

## Release boundary

This is an Unreleased source change only. It does not change version metadata, install a local build over RC.68, create a release candidate, modify a tag/Release, or access GitLab.
