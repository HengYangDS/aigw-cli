## Why

Codex 0.169.0 reports a deprecated configuration key as an error item of its event stream. A client-depth check
counts every such item, so `aigw check codex --depth client` fails on a host whose Codex configuration holds
`[features].multi_agent`, although the session itself succeeds. The check then says only that the client "emitted
a native-client warning or error": it drops the client's text by design, offers no way to read it, also not with
`--verbose`, and names `aigw check` as its Fix. That command starts no client at its default depth, so it passes
and names the failing check again. Whoever runs the check, a person or an agent, cannot tell what to change.

The same reproduction showed that the private Codex home still runs Codex's memory work. It drops the operator's
`[memories]` table but keeps `features.memories`, so each check starts a second thread on another model, whose
request ends early and is retried up to five times beside the session that is being verified.

Showing a client's text made a defect of the redaction owner matter: after `Authorization:` it took the scheme
word for the credential and left the credential visible.

The rule that every failure names one fix allows a command, or a command with placeholders. Neither fits a
failure that no `aigw` command repairs, such as a key in the client's own configuration.

## What Changes

- A client-depth check that stops on a warning or an error of Codex or Claude Code keeps its message free of the
  client's text. It names a configuration key that the client calls deprecated, in the form in which Codex
  writes one, and its Fix is to remove that key.
- `aigw check --verbose` shows the client's own lines for such a failure, after AIGW's own lines in the text and
  as a field of the JSON output: at most four, bounded, without the session's credential and the known
  credential forms. It shows them only for a session whose credential AIGW's store supplies, because only then
  can AIGW take that credential out of them.
- The Fix of such a failure is one sentence and never `aigw check`: what to remove, when AIGW names it; else how
  to read the client's lines; once the output shows them, to resolve what the client reported; and without
  them, to run the client itself.
- **BREAKING** for readers of the JSON output: the check's document has `schema_version` 4, with the new field
  `failure.client_report`; the next step of such a failure is in `next_detail`, and `next_action` is absent.
- A Fix can be one sentence that says what to change, when no `aigw` command repairs the cause.
- The private Codex home turns Codex's memory work off for the session.
- Redaction covers an `Authorization` or a cookie header to the end of its line, a URL's user, a `key`, `sig` or
  `signature` query parameter and a bare `sk-` key.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `model-selection`: a client-depth check shows a client's own report on request, names what to change, and
  runs no memory work of Codex.
- `product-quality`: a warning of a successful native session is reported in AIGW's own words, and the client's
  text appears only on request.
- `command-line`: a Fix can be a sentence, and a document's first `schema_version` is told from its present one.

## Impact

`internal/domain` (redaction), `internal/adapter/verification`, the Codex adapter's session plan,
`internal/cli/check` and `internal/terminal` (a field's quotes). No configuration file of the operator changes.
Redaction is shared, so every caller's output loses the new forms too.

Out of scope: which reports fail a check, and what a successful check proves. A client that authenticates
itself, or reads its Token through an external credential helper, shows no report. Hermes and the version and
catalog probes keep their messages.
