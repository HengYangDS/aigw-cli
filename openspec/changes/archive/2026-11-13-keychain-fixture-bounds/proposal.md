## Why

aigw's GitLab macOS jobs failed twice on 2026-11-13 under host load, and each needed a retry of 10 to 20 minutes.
One failure was a test's own fixture: `TestLockedKeychainMetadataDoesNotPrompt` killed `security create-keychain`
at the test's five-second bound (job 69467). Six test-side bounds in the macOS Keychain tests copy the product's
five-second deadline although they only guard against a hang. `product-quality` already requires such bounds to
leave room for a slow shared CI host.

## What Changes

- Bound the macOS Keychain tests' own `/usr/bin/security` calls, and the child processes they wait for, by one
  named 30-second fixture bound instead of five seconds.
- State in the `secret-storage` requirement that a test's bound around the platform credential tool stops only a
  hang, while the five-second deadline stays the bound of AIGW's own operations.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `secret-storage`: a test's bound around the platform credential tool is a hang bound, separate from the product
  deadline.

## Impact

Two macOS test files and the specification change. The product's deadlines, including `operationTimeout` and the
migration's `securityToolTimeout`, do not change. A regression that makes a read show a UI prompt still fails, after
30 seconds instead of five.
