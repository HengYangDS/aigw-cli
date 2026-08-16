## Context

`os.Chmod(path, 0)` expresses a POSIX permission model. Windows applies a
different ACL and file-attribute model, so the directory can remain readable.
The test was therefore host-dependent even though `checkPackageChildren`
correctly propagates directory-read failures.

## Decision

Keep `checkPackageChildren` as the production entry point. Delegate its
filesystem read to one private helper that accepts the read operation. Runtime
code supplies `os.ReadDir`; the regression supplies a deterministic error.

This is the narrowest portable construction because it proves the intended
error boundary without mutable package globals, host privilege assumptions,
platform skips, or a new filesystem abstraction dependency.

## Rejected Alternatives

| Alternative | Reason |
| --- | --- |
| Skip the test on Windows | Removes required native evidence. |
| Branch on `runtime.GOOS` | Preserves two test meanings instead of one contract. |
| Manipulate Windows ACLs | Adds host policy and privilege dependencies to a unit test. |
| Introduce a virtual filesystem package | Disproportionate dependency and abstraction cost for one private operation. |

## Verification

1. Observe the focused regression fail before the private helper exists.
2. Pass the focused architecture test with the locked toolchain.
3. Pass the complete exact-HEAD proof before integration.
