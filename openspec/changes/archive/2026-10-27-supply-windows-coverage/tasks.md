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

- [x] 3.1 `mise run check` passes on the lane head (`f0f1184daa99850fb2397daee48ee3950cb45adb`, statement
      coverage 95.20%).
- [x] 3.2 The full proof of the lane head passes (Attestation
      `c6610649f3dfcf32655e9dee7de00f7b6432afb91e012a2d46e045bc01d56d50`).
