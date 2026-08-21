## 1. Progressive setup contract

- [x] 1.1 Add configuration tests for deterministic connected-Account profile matching.
- [x] 1.2 Add acceptance tests for zero-Token import, one-Token activation, absent clients, and deferred endpoint health.
- [x] 1.3 Implement catalogue-first setup with explicit or deferred Account connection.
- [x] 1.4 Render imported, connected, selected, projected, and deferred states without credential disclosure.
- [x] 1.5 Restore the canonical team manifest to provider-native Responses endpoints.

## 2. Late client convergence

- [x] 2.1 Add acceptance proving `sync` discovers a client installed after setup.
- [x] 2.2 Reuse one desired-client derivation for synchronization and repair.
- [x] 2.3 Verify repeated synchronization is idempotent and preserves unrelated client state.

## 3. Cross-platform user journey

- [x] 3.1 Prove Windows executable discovery and invocation for native `.exe`, `.cmd`, and `.bat` boundaries on a native Windows runner.
- [x] 3.2 Add one native black-box journey using the built product, isolated user directories, the environment secret backend, setup, projection, check, and uninstall.
- [x] 3.3 Execute that journey on macOS and verify installation, deferred activation, projection, check, uninstall, and retained catalogue state.
- [x] 3.4 Execute that journey on native Linux and native Windows runners.
- [ ] 3.5 Verify the real macOS Keychain, Linux Secret Service, and Windows Credential Manager backends separately; environment-backend evidence MUST NOT stand in for an OS credential backend.

## 4. Documentation and evidence

- [x] 4.1 Rewrite setup and team rollout around the progressive journey and implementation-neutral endpoint dependency.
- [ ] 4.2 Validate OpenSpec strictly and update canonical specifications through archive.
- [x] 4.3 Run focused tests, source gates, native/release gates, exact-asset installation, and runtime verification.
- [ ] 4.4 Publish and verify the same signed product revision on both Forges, then retire the owned lane without residue.
