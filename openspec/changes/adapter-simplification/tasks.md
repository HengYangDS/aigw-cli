## 1. Simplification

- [x] 1.1 Move the Converge preamble into one `internal/adapter` helper, keeping
      Hermes's executable-before-secret order; each adapter's Converge tests pass
      unchanged.
- [x] 1.2 Compare the guarded multi-file writes of Claude Code, Claude Desktop
      and Codex; they differ in rollback order and error text, so each keeps
      its own, as design.md records.
- [x] 1.3 Keep one indented JSON encoder, one raw-message encoder, one receipt
      slice and one state-file suffix in `internal/adapter`; persisted bytes are
      unchanged.
- [x] 1.4 Remove Codex's test-only forwarders and point their tests at the
      functions production calls.
- [x] 1.5 Remove surface metadata that never varies or is never read, keeping
      the persisted `projection_mode` field.

## 2. Verification

- [x] 2.1 Run the local quality graph at the lane head.
- [x] 2.2 Prove the lane head with the full ETHOS proof.
- [x] 2.3 Confirm the hosted three-platform graph at the lane head.
