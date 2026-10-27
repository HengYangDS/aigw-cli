## 1. Host-Specific Retirement

- [x] 1.1 Move the POSIX process inventory of native client retirement into a
      file that builds only for Linux and macOS, with a Windows counterpart
      that makes no inventory. The POSIX retirement test passes, and
      `tools/ci` and its tests build and vet for Windows.

## 2. Coverage

- [x] 2.1 Cover the refusal of a client without an official distribution and of
      a Hermes supply whose private home already exists. Each test fails when
      its refusal is removed and passes with it.

## 3. Verification

- [ ] 3.1 `mise run check` passes on the lane head.
- [ ] 3.2 The full proof of the lane head passes.
