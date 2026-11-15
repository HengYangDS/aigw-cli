## 1. One scope

- [x] 1.1 Open one availability scope where a command's context is built and remove the seven scopes that
      commands open themselves. A table-driven acceptance test runs each command that observes a Token against
      the recording worker and shows that no fact of a slot is asked twice; `provider edit` asked twice before.
- [x] 1.2 Show at command level that a command which stores or removes a Token observes that slot again
      afterwards, and that two commands run one after the other in one process do not share a reading.
- [x] 1.3 Let the undo of a stored Token write through the command's scope on every backend, so that `aigw use`
      states that the Token was not kept also with the backend that AIGW selects; a unit test and a
      command-level test fail without it.
- [x] 1.4 Give the balance credentials' view the command's scope; the table test shows that no fact of a
      balance credential is asked twice, where `provider remove` asked twice before.

## 2. Verification

- [ ] 2.1 `mise run check` passes on the lane head.
- [ ] 2.2 The hosted graph passes at the proposal head on both forges.
