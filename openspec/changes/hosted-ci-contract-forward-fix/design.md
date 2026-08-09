# Design

Hosted jobs provide Forge provenance through environment variables. Unit tests
that exercise the unconfigured local path must explicitly clear those inputs;
tests for missing trust input must likewise own the complete environment they
assert. This keeps the production command unchanged and removes dependence on
the parent process without adding another CI or configuration owner.

The completed Windows log exposes a separate platform-contract gap spanning
native executable fixtures, POSIX permission assertions, path separators,
CRLF-normalized text, and OS-specific archive coordinates. That wider change
requires its own scope and hosted proof after this focused environment fix is
archived.
