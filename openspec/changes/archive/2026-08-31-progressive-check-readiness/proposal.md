## Why

`aigw setup --from` already offers a machine-readable result, while the corresponding readiness command has no machine-readable projection. This makes automation and cross-platform diagnostics unnecessarily dependent on human text and obscures the distinction between configured routes and unavailable optional clients.

## What Changes

- Add a stable `aigw check --json` projection for the same read-only readiness facts shown by `aigw check`.
- Keep readiness scoped to enabled, selected client routes; catalogue entries and unavailable optional clients remain non-blocking.
- Use one readiness evaluation model for human and JSON output; do not add a second diagnostic path.
- **BREAKING**: none; the existing human command remains unchanged.

## Capabilities

### New Capabilities

- `cli-readiness`: Machine-readable and human-readable readiness projections share one evaluation result.

### Modified Capabilities

- `progressive-team-onboarding`: The deferred setup journey exposes a deterministic next action through the same readiness facts used by `check --json`.

## Impact

Affected files are the readiness command, its acceptance tests, and the CLI readiness specification. No provider, proxy, credential, or ETHOS lifecycle behavior changes.
