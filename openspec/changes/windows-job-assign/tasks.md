## 1. Containment

- [x] 1.1 Start a captured Windows child suspended, assign it to its Job, then
      resume it, keeping the caller's creation flags; tests pin the order and
      the flags, and stop a child that cannot resume.

## 2. Verification

- [x] 2.1 Run the local quality graph at the lane head, including the Windows
      cross-compile and vet.
- [x] 2.2 Prove the lane head with the full ETHOS proof.
- [x] 2.3 Confirm the hosted three-platform graph at the lane head, with the
      Windows job running the process and credential tests.
