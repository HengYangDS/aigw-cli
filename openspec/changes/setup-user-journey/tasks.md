## 1. Contract

- [x] 1.1 Reproduce the missing manifest-setup JSON surface and verify the
      current command rejects `--json`.
- [x] 1.2 Define the minimal progressive-onboarding delta and verify
      `openspec validate setup-user-journey --strict` recognizes it.

## 2. Implementation

- [x] 2.1 Add a secret-free manifest-setup result shared by human and JSON
      rendering; verify the focused setup acceptance tests pass.
- [x] 2.2 Exercise zero-Token, one-Token, absent-client, and later-installed
      client journeys in isolated homes and verify no external endpoint lifecycle
      is claimed.

## 3. Closeout

- [x] 3.1 Run the affected source and policy gates and produce exact-HEAD proof.
- [x] 3.2 Archive the completed official Change, land it through the governed
      path, and retire the work lane without leaving a proposal or work ref.
