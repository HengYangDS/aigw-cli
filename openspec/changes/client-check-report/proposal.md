## Why

Codex 0.169.0 reports a deprecated configuration key as an error item of its event stream. A client-depth check
counts every such item, so `aigw check codex --depth client` fails on a host whose Codex configuration holds
`[features].multi_agent`, although the session itself succeeds. The check then says only that the client "emitted
a native-client warning or error": it drops the client's text by design, offers no way to read it, also not with
`--verbose`, and names `aigw check` as its Fix, which fails the same way. Whoever runs the check, a person or an
agent, cannot tell what to change.

The same reproduction showed that the private Codex home still runs Codex's memory work. It drops the operator's
`[memories]` table but keeps `features.memories`, so each check starts a second thread on another model, whose
request ends early and is retried up to five times beside the session that is being verified.

Showing a client's text made a defect of the redaction owner matter: after `Authorization:` it took the scheme
word for the credential and left the credential visible.

## What Changes

- A client-depth check that stops on a warning or an error of Codex or Claude Code keeps its message free of the
  client's text. It names a configuration key path that the client calls deprecated, and its Fix is to remove
  that key.
- `aigw check --verbose` shows the client's own lines for such a failure, in the text and in the JSON output: at
  most four, bounded, with the known credential forms and the provider's Token redacted. Without the flag, the
  Fix says how to read them. No Fix of such a failure is a command that fails the same way.
- **BREAKING** for readers of the JSON output: the check's document has `schema_version` 4, with the new field
  `failure.client_report`; the next step of such a failure is prose in `next_detail`.
- The private Codex home turns Codex's memory work off for the session.
- Redaction covers an `Authorization` header with its scheme, a cookie line, a URL's user and password, a `key`
  query parameter and a bare `sk-` key.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `model-selection`: a client-depth check shows a client's own report on request, names what to change, and
  runs no memory work of Codex.
- `product-quality`: a warning of a successful native session is reported in AIGW's own words, and the client's
  text appears only on request.

## Impact

`internal/domain` (redaction), `internal/adapter/verification`, the Codex and Claude adapters and
`internal/cli/check`. No configuration file of the operator changes.

Out of scope: which reports fail a check, and what a successful check proves. A client that reads its Token
through an external credential helper shows no report, because the helper's output can stand in what the client
prints; Hermes, whose verification treats its output that way, and the version and catalog probes keep their
messages.
