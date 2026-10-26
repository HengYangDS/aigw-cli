## 1. Client Supply

- [x] 1.1 Write supply, admission, interruption and retirement tests for each
      supported host. They fail against the Windows-only supply.
- [x] 1.2 Make `ci native-clients` supply, for its own host, the clients that
      the release owner declares, at the pinned official releases. Keep their
      state confined and prove its retirement. The client tests pass.
- [x] 1.3 Run every `ci` command through the native process owner, so that a
      signal to the supplier stops its running installer and every process the
      installer started before the supply is removed. A real blocking installer
      that ignores SIGTERM passes (BLOCK `54dfadb0`).
- [x] 1.4 Give the official Hermes installer a declared environment, with its
      tool store, `HERMES_RUNTIME_DIR`, inside the supply. A job's
      `HERMES_RUNTIME_DIR` no longer reaches it (BLOCK `5296f324`).

## 2. Pipeline

- [x] 2.1 Pin the client releases once in the CUE model. Declare the supply,
      retirement, `native_clients` input and client tool closure for every
      GitHub native job. Regenerate both Forge projections. The projection
      tests pass.
- [x] 2.2 Describe the per-host supply and the renamed input in
      `CONTRIBUTING.md`.
- [x] 2.3 Before the supply, prepare Codex's documented Linux sandbox
      prerequisites in every GitHub Linux job that requests clients: the
      distribution's bubblewrap and its AppArmor profile. Without them, run
      `40411171963` failed the Linux Codex journey. The projection tests pass.

## 3. Verification

- [x] 3.1 `mise run check` passes on the lane head (`72302989`, statement
      coverage 95.19%).
- [x] 3.2 Dispatch GitHub Verify on the proposal head with
      `native_platform=linux`, `native_clients=true` and a published baseline
      and candidate. The retained-predecessor journey passes with Claude, Codex
      and Hermes, and retirement proves the supply absent (run `40418580826`
      at `690d3376`, v0.4.16 to v0.4.17, no skip).
- [x] 3.3 The same dispatch passes on macOS with all three clients, and on
      Windows with Claude and Codex (runs `40418594141` and `40418607327` at
      `690d3376`, no skip).
- [ ] 3.4 The full proof of the lane head passes.
