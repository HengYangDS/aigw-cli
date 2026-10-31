## 1. Residue

- [x] 1.1 Make a helper process inherit the Git isolation of the test process
      that started it; the regression test whose helper exits inside a test
      fails before the change and passes after it, with an empty temporary
      directory.
- [x] 1.2 Run the program's entry-point test through the `aigw` command that
      the script harness installs; the package's tests leave no
      `testscript-main*` directory.

## 2. Verification

- [x] 2.1 `mise run check` with TMPDIR set to an empty directory leaves only
      Node's `node-compile-cache` there, `mise run dependencies:check` passes,
      and the full proof of the lane head passes.
- [x] 2.2 The hosted graph passes at the lane head.
