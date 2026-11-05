## 1. Refresh

- [x] 1.1 Advance the repository npm to 12.2.2 in the Mise declaration, and
      regenerate its lock entry and aube graph with `mise lock`; OSV reports
      no advisory for the bundled undici and ip-address.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head (f8eef5f5).
- [x] 2.2 The hosted graph passes at the proposal head (PR 237).
