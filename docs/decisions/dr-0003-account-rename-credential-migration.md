# DR-0003: Migrate Account Rename Credentials in Two Phases

- Status: accepted
- Date: 2026-07-24
- Owner: AIGW maintainers

## Context

Renaming a provider Account ID requires migrating both its configuration in the AIGW TOML and its associated credentials (API Tokens and optional probe credentials) in the operating-system secret store. A single-step rename that immediately deletes the old credential slot risks permanent loss of access or inconsistent state if the configuration write is interrupted or if a client integration remains bound to the old ID.

The API-token slot and optional provider-diagnostic slot in the selected AIGW credential backend cannot be committed atomically with configuration. Phase 1 therefore retains the old slots; finalization separately verifies the current configuration, its single `.bak` preimage, and the complete admitted-client verified checkpoint.

## Decision

Implement a two-phase "copy-then-delete" migration for `aigw account rename`:

### Phase 1: Configuration and Credential Adoption

`aigw account rename [old] [new]` performs the following:

1. **Configuration Migration**: Moves the Account key in the TOML configuration and updates all `Profile.Account` references to the new ID.
2. **Credential Adoption**: For the Token and optional account-probe credential, a missing target slot is copied from the old slot and read back before configuration is committed.
3. **Fail-Closed Consistency**:
   - If a target credential slot already contains the same value, the migration is resumable.
   - If a target slot contains a different value, the command fails closed without changing the source configuration or old slot.
   - If the `env` backend is active, equal target environment variables must be externally pre-provisioned because that backend is read-only.
4. **Retention**: After success, the old Account key is absent from the current TOML and remains only in the single `.bak` configuration preimage. Phase 1 never deletes the old credential slots, whether it succeeds or fails.
5. **Dry-Run**: `--dry-run` renders the plan without taking the mutation lock or writing configuration, `.bak`, credentials, or client state.

### Phase 2: Finalization and Cleanup

`aigw account rename <old> <new> --finalize` performs the following:

1. **Strict Verification**: Requires explicit old and new IDs, semantic agreement between the current configuration and the complete admitted-client verified checkpoint, and an available target Token.
2. **Backup Convergence**: Before deleting an old slot, uses a three-file exact-preimage check to converge the single `.bak` to the verified current TOML.
3. **Rotation Confirmation**: Requires `--confirm-api-token-rotation` only when the old and new Token slots differ, and `--confirm-account-probe-rotation` only when the corresponding probe slots differ.
4. **Probe Execution**: If probe credentials differ during apply, executes the target provider probe live before cleanup.
5. **Credential Cleanup**: Removes the old credential slots only after the preceding checks. Partial deletion is retryable; with the `env` backend, old variables must be unset externally and the non-zero, incomplete finalization retried.
6. **Idempotency**: Dry-run JSON after successful finalization reports `already-finalized`.

## Consequences

- **Safety**: Phase 1 is resumable and keeps the old credential slots; explicit finalization and retryable partial cleanup separate adoption from deletion.
- **Non-atomic boundary**: The configuration, Token secret store, and account-probe secret store do not form an ACID transaction. The three-file exact-preimage check is best-effort race protection, not a cross-process CAS; detected competing changes fail closed and may require a retry.
- **Visibility**: JSON machine output and human-readable reports remain secret-free and path-free; no credentials or local filesystem paths enter the rename records.
- **Constraints**: Labels, model IDs, endpoints, and probe kinds do not change with the Account ID; renaming is strictly an identity migration.

## Evidence

Regression suites cover:

- 0/1/2 argument handling in interactive and non-interactive modes.
- Token and probe credential copying and comparison logic.
- Fail-closed behavior for credential mismatches and `env` backend requirements.
- `--finalize` consistency checks against verified checkpoints.
- Single `.bak` convergence, conditional rotation confirmation, and retryable old-slot removal.
- Dry-run and JSON output integrity.

## Revisit Trigger

Revisit this decision if a cross-process CAS (Compare-And-Swap) guarantee is implemented across configuration and both secret stores or if the configuration-and-checkpoint transaction model is superseded by a centralized registry.
