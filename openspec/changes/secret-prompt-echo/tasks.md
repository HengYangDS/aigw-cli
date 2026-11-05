## 1. Fix

- [x] 1.1 Read accessible Token prompts with terminal echo disabled and decide
      accessibility once from the CLI environment; a regression test proves a
      terminal Token never reaches the prompt output or the plain line reader.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head.
- [ ] 2.2 The hosted graph passes at the proposal head.
