## 1. Cleanup

- [x] 1.1 Count a verification home with a gateway launch log as owned until
      Hermes's lifecycle record shows the gateway exited or its process gone;
      tests cover a starting, running, exited, dead-running and unreadable
      lifecycle record.
- [x] 1.2 Record the fix in CHANGELOG's `[Unreleased]` section.

## 2. Verification

- [ ] 2.1 Run the local quality graph at the lane head.
- [ ] 2.2 Run `aigw verify --for hermes` from a lane build against the real
      Hermes, and confirm that its home is gone and no gateway outlives it.
- [ ] 2.3 Prove the lane head with the full ETHOS proof.
- [ ] 2.4 Confirm the hosted three-platform graph at the lane head.
