## 1. Gate

- [ ] 1.1 Let the OpenSpec gate pass an active Change that declares `skip_specs: true`, holds no delta and has
      the validator's acknowledgement as its only finding, and name each such Change in the gate's summary. A
      test drives the gate with a report for each case of the scenario "A Change declares that it changes no
      specification" (declared; declared with another finding; acknowledged without the declaration; another
      `INFO`) and fails without the change.

## 2. Verification

- [ ] 2.1 `mise run check` passes on the lane head.
- [ ] 2.2 The hosted graph passes at the proposal head on both forges.
