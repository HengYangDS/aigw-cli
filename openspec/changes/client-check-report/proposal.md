## Why

Codex 0.169.0 reports a deprecated configuration key as an error item of its event stream. A client-depth check
counts every such item, so `aigw check codex --depth client` fails on a host whose Codex configuration holds
`[features].multi_agent`, although the session itself succeeds. The check then says only that the client "emitted
a native-client warning or error": it suppresses the client's text by design and offers no way to read it, also
not with `--verbose`. Whoever runs the check, a person or an agent, cannot tell what to change.

The same reproduction showed that the private Codex home still runs Codex's memory work. It drops the operator's
`[memories]` table but keeps `features.memories`, so each check starts a second thread on another model, whose
request fails and is retried five times beside the session that is being verified.

## What Changes

- A client-depth check that stops on a client's own warning or error keeps its message free of the client's
  text, and names a configuration key that the client calls deprecated.
- `aigw check --verbose` shows the client's own lines for such a failure: at most four, bounded, with credentials
  redacted, in the text and in the JSON output. The fix of that failure is the same check with `--verbose`.
- Hermes and the two Codex probes report through the same owner, so every client's warning can be read this way.
- The private Codex home turns Codex's memory work off for the session.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `model-selection`: a client-depth check shows a client's own report on request, and runs no memory work of
  Codex.
- `product-quality`: a warning of a successful native session is reported in AIGW's own words.
- `cli-readiness`: a warning symbol is reported without the client's text.

## Impact

`internal/adapter/verification`, the Codex and Hermes adapters and `internal/cli/check`. The JSON output gains one
optional field, `failure.client_report`, present only with `--verbose`. No configuration file of the operator
changes. The out-of-scope
boundary: which reports fail a check stays as it is, and so does what a successful check proves.
