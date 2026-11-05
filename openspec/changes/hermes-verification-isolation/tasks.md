## 1. Isolation

- [x] 1.1 Project no MCP servers, no shell hooks and no hook consent into the
      isolated Hermes verification home; a regression test proves operator
      MCP servers, hooks and hook consent never reach it and the operator's
      configuration stays unchanged.
- [x] 1.2 Record the fix in CHANGELOG's `[Unreleased]` section.

## 2. Verification

- [ ] 2.1 Run the local quality graph at the lane head.
- [ ] 2.2 Run `aigw verify --for hermes` from a lane build against the real
      Hermes, and confirm that the verification home starts no operator MCP
      server.
- [ ] 2.3 Prove the lane head with the full ETHOS proof.
- [ ] 2.4 Confirm the hosted three-platform graph at the lane head.
