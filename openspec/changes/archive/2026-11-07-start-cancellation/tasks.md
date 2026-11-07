## 1. Report

- [x] 1.1 Join the context's error into a failed captured start in
      `Runner.RunStream`; a Windows test that ends the context while the child
      is suspended receives the cancellation.

## 2. Verification

- [x] 2.1 Run the local quality graph at the lane head, including the Windows
      cross-compile and vet. Check `b1` passed at `6a7e0a3b`.
- [x] 2.2 Confirm the hosted three-platform graph at the lane head, with the
      Windows job running the process tests. PR 248's Verify run 42372212962
      passed at `6a7e0a3b`.
