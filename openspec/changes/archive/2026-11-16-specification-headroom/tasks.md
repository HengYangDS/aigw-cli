## 1. Move

- [x] 1.1 Move the six requirements on the operating system's credential service from `secret-storage` to a new
      `native-credential-store`, the four of a Hermes verification, the one of a real Codex session and the one on
      warning symbols from `cli-readiness` to a new `client-verification`, the seven on output from `command-line`
      to a new `command-output`, and the four on quantitative evidence from `product-quality` to a new
      `quantitative-quality`; each new specification states its purpose, `cli-readiness` and `product-quality`
      lose the clause of their purpose that moved, and `command-line` names what it holds.
- [x] 1.2 Show that every requirement block of the four sources at the base stands byte for byte in exactly one
      specification at the head, that no other specification changed, and that every specification validates
      strictly and holds fewer than 400 lines among the eight.

## 2. Verification

- [x] 2.1 `mise run check` passes on the lane head.
- [x] 2.2 The hosted graph passes at the proposal head on both forges.
