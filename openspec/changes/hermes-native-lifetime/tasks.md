# Tasks

## 1. Gateway Lifetime

- [x] 1.1 Make configuration tests expect the 2-second idle exit in the
      verification policy and `GatewayAbsent` to report a held, released and
      missing gateway lock; they fail before the change.
- [x] 1.2 Make client tests expect the verification environment's own lock
      directory, deletion only after the lock is free, one native stop with
      closed input after the idle bound, and a named retained home when the
      lock stays held; they fail before the change.
- [x] 1.3 Add the idle exit, the lock directory, `GatewayAbsent`, the bounded
      release with one native stop, and the remover's lock check; the tests
      pass.

## 2. Diagnostics

- [x] 2.1 Make classifier tests expect line-leading `⚠`, `✗`, `✖`, `✘` and
      `❌` to fail, with check marks, `↻`, `session_id:` and inner symbols as
      ordinary controls; they fail before the change.
- [x] 2.2 Recognize the line-leading symbols in `DiagnosticFailure`; the tests
      pass.

## 3. Release Journey

- [x] 3.1 Create the journey's Hermes session once, resume it by its stored
      identifier without frozen creation options, and stop the journey gateway
      before its home is removed.
- [ ] 3.2 Run the repaired Hermes native client journey with the installed
      Hermes.

## 4. Verification

- [x] 4.1 Record the change in `CHANGELOG.md`.
- [ ] 4.2 `mise run check` and the full proof of the lane head pass.
