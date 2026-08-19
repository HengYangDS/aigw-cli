## Context

The Windows runner evaluates `filepath.IsAbs` with Windows semantics. POSIX literals such as `/usr/local/bin/aigw` are therefore intentionally invalid.

## Decision

Use `filepath.Join` with temporary or stable test roots so fixtures follow the executing platform. Compare projected helpers through the existing `credentialHelper` owner instead of duplicating shell grammar in test literals.

## Non-goals

- no relaxation of absolute-path validation;
- no CI exception, retry, timeout, or platform skip;
- no production-code or client-state mutation.
