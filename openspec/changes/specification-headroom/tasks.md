## 1. Move

- [x] 1.1 Move the five requirements on the operating system's credential service from `secret-storage` to a new
      `native-credential-store`, the four of a Hermes verification and the one of a real Codex session from
      `cli-readiness` to a new `client-verification`, the seven on output from `command-line` to a new
      `command-output`, and the four on quantitative evidence from `product-quality` to a new
      `quantitative-quality`; each new specification states its purpose, and three sources lose the clause of
      their purpose that moved.
- [x] 1.2 Show that every requirement block of the four sources at the base stands byte for byte in exactly one
      specification at the head, that no other specification changed, and that every specification validates
      strictly and holds fewer than 400 lines among the eight.

## 2. Verification

- [ ] 2.1 `mise run check` passes on the lane head.
- [ ] 2.2 The hosted graph passes at the proposal head on both forges.
