## 1. Process Runner

- [x] 1.1 Tell a plan's caller when the operating system has created the child, before the start observation; a test
      with a real child shows that order on every platform.
- [x] 1.2 On Windows, make that observation after the suspended creation and before the Job assignment and the resume,
      and never when the creation fails; a test with the scripted Windows API shows both.

## 2. Native Credential Deadline

- [x] 2.1 Return from a native credential operation that reaches its deadline an error that is still a deadline and
      that names the step with each time; a table covers every combination of the two marks without a clock, tests
      with a scripted runner cover no mark, creation alone, creation and start, and marks that arrive after the
      deadline, and a cancelled caller is not reported as a deadline.
- [x] 2.2 Show where the operation stood as the evidence of the credential command's deadline problem and name the
      credential service in its fix only when the subprocess was started; its test reads both for each case.

## 3. Records

- [x] 3.1 State the change under Unreleased in `CHANGELOG.md`.

## 4. Verification

- [x] 4.1 Compile and vet the three changed packages and their tests for Windows on the lane head, since their
      Windows tests run only on the forges.
- [x] 4.2 Pass `mise run check` on the lane head.
- [x] 4.3 Pass the proposal's hosted checks on both forges, the Windows jobs included.
