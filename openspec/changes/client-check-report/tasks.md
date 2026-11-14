## 1. The report

- [ ] 1.1 Return AIGW's message, a remedy and the client's output as three things from `verification.Diagnostic`,
      and name a deprecated configuration key path by its grammar; its tests show that no message or remedy
      carries the client's text.
- [ ] 1.2 Bound and redact the report: at most four lines of 300 characters, the known credential forms and the
      reader's secrets removed; its tests cover each bound and each form.
- [ ] 1.3 Redact an `Authorization` header with its scheme, a cookie line, a URL's user and password, a `key`
      query parameter and a bare `sk-` key in the redaction owner; a table test holds each form.
- [ ] 1.4 Show the report with `--verbose`, under the failing client's row and as `failure.client_report` of the
      JSON output, with `schema_version` 4; tests show the lines with the flag and none without it.
- [ ] 1.5 Give such a failure a Fix that is not the same check; a test holds the four cases.
- [ ] 1.6 Withhold the report of a client that reads its Token through an external helper; a test shows that no
      reader finds the report.

## 2. The private Codex home

- [ ] 2.1 Turn Codex's memory work off on the session's command line; a test finds `features.memories=false`
      among the plan's arguments of a first and a resumed turn.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 3.2 The whole-program script passes: the built program, in a private home, against a scripted client that
      reports a deprecated key and then a warning with a credential.
- [ ] 3.3 The hosted graph passes at the proposal head, and the fresh-context reviews of the lane are read.
