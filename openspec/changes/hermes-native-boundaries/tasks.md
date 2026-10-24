# Tasks

## 1. Gateway Process Exit

- [x] 1.1 Make configuration and client tests expect a home whose recorded
      gateway process still runs to be kept after its lock is free, and deleted
      once that process exits; they fail before the change.
- [x] 1.2 Require the recorded gateway process to have exited in
      `GatewayAbsent`; the tests pass.

## 2. Session Route Note

- [x] 2.1 Make selection tests expect the "Existing sessions" row for Hermes
      when its projection changes or would change, and no row otherwise; they
      fail before the change.
- [x] 2.2 Add `SessionsKeepRoute` to the client registry and the row to
      `aigw use` and its preview; the tests pass.

## 3. Verification

- [x] 3.1 Record the change in `CHANGELOG.md`.
- [ ] 3.2 Run the Hermes native client journey with the installed Hermes.
- [ ] 3.3 `mise run check` and the full proof of the lane head pass.
