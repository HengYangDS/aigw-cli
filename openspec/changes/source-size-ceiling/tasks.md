## 1. Ceiling

- [ ] 1.1 Lower `max_code_lines` and `max_specification_lines` in
      `.config/checks/scc/policy.toml` to 500, with the gate's boundary tests
      at 500 and 501 lines; `check-source-size` and the `tools/ci` tests pass.
- [ ] 1.2 Move one concern's tests out of each of the ten Go test files above
      500 effective lines; every file holds 500 or fewer, and the tests of each
      touched package pass on macOS, Linux and Windows vet.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head, the full proof of the lane head passes, and the ETHOS alpha.23
      readiness proof reports no size gap.
- [ ] 2.2 The hosted graph passes at the lane head.
