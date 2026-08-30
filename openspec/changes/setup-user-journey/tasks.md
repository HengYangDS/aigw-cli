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
- [x] 3.2 Confirm the completed Change is ready for the governed archive and
      landing transition; retire its temporary refs during lifecycle closeout.

## 4. Client Route Authority

- [x] 4.1 Specify and reproduce the global-default contradiction after two
      client-specific selections.
- [x] 4.2 Replace default-plus-overrides with one explicit client Route map and
      migrate the previous local schema without retaining parallel semantics.
- [x] 4.3 Remove ambiguous `use --all`, default inheritance, and reset behavior;
      make Profile client identity the command's selection authority.
- [x] 4.4 Make setup, check, status, doctor, sync, credentials, rename, recovery,
      import, and export consume the same Route model.
- [x] 4.5 Prove two independently selected clients, one selected client, no
      installed client, migration, and manifest round-trip behavior.
- [x] 4.6 Re-run strict OpenSpec validation, affected tests, source gate, real
      CLI journey, and exact-HEAD proof before archive.
