## 1. The report

- [ ] 1.1 Make `verification.Diagnostic` return AIGW's message and the client's bounded, redacted report as two
      things, and name a deprecated configuration key by its grammar; its tests show that no message carries the
      client's text and that a report is at most four lines of 300 characters.
- [ ] 1.2 Report Hermes's warnings and the two Codex probes through the same owner; their tests keep refusing a
      leaked credential in the message.
- [ ] 1.3 Show the report with `--verbose`, under the failing client's row and as `failure.client_report` of the
      JSON output, and name the same check with `--verbose` as the fix; tests show the lines with the flag and
      none without it, in both outputs.

## 2. The private Codex home

- [ ] 2.1 Turn Codex's memory work off on the session's command line; a test finds `features.memories=false`
      among the plan's arguments, and one session on the operator host sends requests for the verified model only.

## 3. Verification

- [ ] 3.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 3.2 The whole-program script runs the built program in a private home against a scripted client that
      reports a deprecated key: the message names the key, `--verbose` shows the redacted line in both outputs,
      and the adapter's test reads Codex 0.169.0's own item as it arrived on the operator host.
- [ ] 3.3 The hosted graph passes at the proposal head, and a fresh-context review of the lane is read.
