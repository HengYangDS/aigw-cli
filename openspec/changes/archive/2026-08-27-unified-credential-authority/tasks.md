## 1. Contract

- [x] 1.1 Define one selected backend as the authority for every credential purpose.
- [x] 1.2 Define typed slot isolation and collision-free environment projection.
- [x] 1.3 Preserve zero-or-one-Account setup and deferred client activation.

## 2. Implementation

- [x] 2.1 Add typed credential views to every supported backend.
- [x] 2.2 Replace the diagnostic-only keyring store with the selected backend.
- [x] 2.3 Enforce canonical lowercase Account IDs and document environment names.
- [x] 2.4 Keep team setup optional per Account and actionable when clients are absent.

## 3. Verification

- [x] 3.1 Pass focused secret, account, configuration, CLI, and CI-tool tests.
- [x] 3.2 Pass the locked static, governance, behavior, and coverage gates.
- [x] 3.3 Pass the native macOS build, install, setup, and uninstall journey.

## Post-archive lifecycle

Produce exact-HEAD Linux and Windows native evidence, dual-Forge CI evidence,
and release acceptance for the archived product tree. Hosted verification and
publication are external effects of the accepted source and do not keep its
completed design Change active.
