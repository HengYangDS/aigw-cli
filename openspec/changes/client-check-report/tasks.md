## 1. The report

- [ ] 1.1 Return AIGW's message, a remedy and the client's output as three things from `verification.Diagnostic`,
      and name a deprecated configuration key in Codex's form only; its tests show that no message or remedy
      carries the client's text, and that a path without a table is not named.
- [ ] 1.2 Give the report only to a reader who names the session's credential, and bound it: at most four lines
      of 300 characters, each redacted once; its tests cover each bound, a credential that a rule cuts short and
      a reader without a credential.
- [ ] 1.3 Redact an `Authorization` or a cookie header to the end of its line, a URL's user, a `key`, `sig` or
      `signature` query parameter and a bare `sk-` key in the redaction owner; a table test holds each form and
      what each must leave.
- [ ] 1.4 Show the report with `--verbose`, as quotes after AIGW's own lines and as `failure.client_report` of
      the JSON output, with `schema_version` 4; tests show the lines with the flag, none without it, and a
      wrapped quote that continues deeper than a detail.
- [ ] 1.5 Give such a failure a Fix that is one sentence and never `aigw check`; a test holds every case of
      remedy, report and flag.
- [ ] 1.6 Read the report with the Token of the endpoint depth only; tests show no report for a session whose
      credential AIGW does not hold.

## 2. The private Codex home

- [ ] 2.1 Turn Codex's memory work off on the session's command line; a test finds `features.memories=false`
      among the plan's arguments of a first and a resumed turn.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 3.2 The whole-program script passes: the built program, in a private home, against a scripted client that
      reports a deprecated key, a warning with a credential, the provider's Token, and a warning under an
      external helper.
- [ ] 3.3 The hosted graph passes at the proposal head.
- [ ] 3.4 Each finding of the fresh-context reviews of the lane is fixed on the lane head or answered in the
      design.
