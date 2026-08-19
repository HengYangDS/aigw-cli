## Context

`filepath.IsAbs` intentionally follows the executing host, so
`/usr/local/bin/aigw` is not absolute on Windows. POSIX mode bits are likewise
not a portable way to force `git receive-pack` failure, and root may write a
directory after `chmod 0500`.

## Decision

Construct executable fixtures with `filepath.Join(t.TempDir(), "aigw")` and
compare generated Claude helper text with the production encoder. Make the
Forge rejection deterministic by installing a failing `pre-receive` hook in
the test peer.

## Non-goals

- no relaxation of executable-path validation;
- no platform skip, retry, timeout extension, or ignored failure;
- no production implementation change.
