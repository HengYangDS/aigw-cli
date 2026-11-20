## 1. The Anthropic model behind an ID

- [x] 1.1 Add the domain function that returns, for a model, the Anthropic model ID that it serves: its family when
      that family begins with `claude-` and is not the model's own ID, or nothing. Done when its table test (a
      channel by prefix and by suffix, a model that is its own family, a non-Claude family, an empty family) passes.
      Done in `23187c51b`.

## 2. Claude Code

- [x] 2.1 Project `modelOverrides` into Claude Code's settings from the selected provider's Claude models, the
      selected model's ID winning its family, as an owned value that is recorded, restored on withdrawal and
      refused as drift when edited outside AIGW. Done when the adapter tests for the map, an empty map, the
      selected channel, withdrawal, an outside edit and a settings file without the key pass.
      Done in `23187c51b`.
- [x] 2.2 Show in the real Claude Code, against a local endpoint that records each request's model, that every
      picker alias sends the provider's ID and that `/context` gives Fable 5.2 a 1M window. Done when the
      recorded run names each sent ID. Recorded in the main checkout's
      `build/evidence/claude-model-windows-20261120/`.

## 3. Claude Desktop

- [x] 3.1 Write `behavesAs` on each Desktop model entry that serves another Anthropic model ID, as part of the
      owned profile. Done when the adapter tests for an entry with and without it, and for its change as managed
      drift, pass. Done in `23187c51b`.
- [x] 3.2 Show in Desktop's own Claude Code, with Desktop's 200K variable set, that a `claude-code-fable-5-2` entry
      with its `behavesAs` row gets a 1M window. Done when the recorded run shows 1m with the row and 200k
      without. Recorded in the main checkout's `build/evidence/claude-model-windows-20261120/`.

## 4. Verification

- [ ] 4.1 Run the changed packages' tests, the linter and the secret scan in the lane. Done when each passes.
- [ ] 4.2 Full exact-HEAD proof of the lane head passes.
