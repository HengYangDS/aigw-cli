## 1. Simplification

- [ ] 1.1 Move the Converge preamble into one `internal/adapter` helper, keeping
      Hermes's executable-before-secret order; each adapter's Converge tests pass
      unchanged.
- [ ] 1.2 Move the guarded multi-file write with reverse rollback beside the
      platform's guarded file functions and use it from Claude Code, Claude
      Desktop and Codex; each adapter's write, refusal and rollback tests pass
      unchanged.
- [ ] 1.3 Keep one indented JSON encoder, one raw-message encoder, one receipt
      slice and one state-file suffix in `internal/adapter`; persisted bytes are
      unchanged.
- [ ] 1.4 Remove Codex's test-only forwarders and point their tests at the
      functions production calls.
- [ ] 1.5 Remove surface metadata that never varies or is never read, keeping
      the persisted `projection_mode` field.

## 2. Verification

- [ ] 2.1 Run the local quality graph at the lane head.
- [ ] 2.2 Prove the lane head with the full ETHOS proof.
- [ ] 2.3 Confirm the hosted three-platform graph at the lane head.
