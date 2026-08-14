## Decision

Use real filesystem shapes whenever they deterministically express the failure:
a regular file blocking a directory component, an invalid path for `Stat`, a
directory blocking atomic replacement, and a non-empty directory blocking the
self-update backup path.

Removal failures cannot be constructed portably after the guarded snapshot has
succeeded. Keep `os.Remove` as the production operation, but pass it through a
private helper so an internal test can prove error propagation without mutable
package globals or platform ACLs.

The configuration package does not repeat the atomic temporary-file failure
test owned by `internal/transaction`; it retains configuration-specific wrapping
and backup failure coverage.

## Verification

1. Reproduce the former failures as root in the locked Linux environment.
2. Pass focused configuration, transaction, and upgrade tests in that same
   environment.
3. Pass the complete source graph and exact-HEAD proof before landing.
