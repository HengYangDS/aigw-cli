# Tasks

## 1. Keep disabled bindings out of readiness

- [x] 1.1 Add a failing regression: with Codex enabled on a healthy Account and
      Claude Code selected but disabled on another Account, making that
      Account's credential metadata unobservable changes `status`, `check` and
      `doctor` output today, and readiness reads that Account.
- [x] 1.2 Read a disabled binding's Account only through activation in the
      client inspection of `internal/cli/readiness`; the regression and every
      existing readiness, status, check and doctor test pass.
- [x] 1.3 Add the user-visible fix to `CHANGELOG.md` under `## Unreleased`.

## 2. Verification and release

- [ ] 2.1 `mise run check` passes, and the lane head is proved and landed
      locally.
- [ ] 2.2 With user authorization, release the fix as 0.4.3 through the
      existing release workflow, publishing each peer's Release from the same
      local bundle; after the Homebrew upgrade, the operator's `aigw status`
      and `aigw doctor` keep reporting the enabled clients.
