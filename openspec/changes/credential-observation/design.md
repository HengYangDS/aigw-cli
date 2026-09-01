## Context

See [proposal.md](proposal.md). `secrets.Store.Has` currently calls `Get`; its
Boolean result both reads Token bytes and collapses every backend error into
"missing". The selected backend is already the sole credential authority and
must remain so.

## Goals / Non-Goals

**Goals:**

- Give every backend one error-bearing availability operation.
- Use metadata-only native queries where the platform exposes them.
- Keep actual value reads at operations that consume a credential.

**Non-Goals:**

- Replacing the stable value-storage dependency without demonstrated net
  benefit.
- Caching availability, probing multiple backends, or persisting a parallel
  credential index.
- Changing client projections or introducing a Proxy dependency.

## Decisions

### One store operation returns presence or an error

Replace `Has(string) bool` with `Exists(string) (bool, error)`, including typed
credential slots. This is the minimal complete domain result: absence is a
valid observation; inability to observe is an error. A three-state enum would
duplicate Go's established value-plus-error semantics.

Alternatives rejected:

- Retaining `Has` and adding `Exists`: parallel semantics and continued misuse.
- Returning only an error: conflates absence with observation failure.
- Caching successful writes: creates a second authority and cannot describe
  credentials created or removed outside AIGW.

### Native metadata observation belongs behind the existing Keyring store

Value operations continue through the current stable keyring library.
Availability uses the narrowest native metadata operation on each platform:
macOS `security find-generic-password` without `-w`, Linux Secret Service
attribute search without `GetSecret`, and Windows `CredEnumerateW` while
reading only `TargetName` before releasing the native allocation. AIGW never
copies or dereferences the returned credential blob. Platform files keep this
difference out of the domain and CLI layers.

Replacing the dependency was rejected: assessed alternatives either lack a
cross-platform non-retrieving existence API, add unrelated backend surface, or
are not mature enough to reduce lifecycle risk.

### Callers either propagate observation errors or deliberately consume values

Read-only reporting surfaces preserve an explicit observation issue. Actions
that require a credential fail before prompting, projecting, or mutating when
observation fails. Network authentication reads the value once directly; it
does not first perform a redundant availability query.

## Risks / Trade-offs

- **Native metadata behavior differs by operating system** → isolate it in
  build-tagged implementations and verify native journeys on all supported
  systems.
- **Some callers previously treated every error as missing** → migrate the
  interface atomically and add failure-path tests before implementation.
- **Linux locked collections can still reject metadata access** → return the
  backend error; never prompt or silently fall back during observation.

## Migration Plan

1. Add failing contract and caller tests.
2. Replace `Has` atomically across implementations and callers.
3. Verify focused tests, native platform journeys, then the exact-HEAD product
   gate.
4. Roll back the single commit if any supported platform cannot provide a
   truthful non-interactive observation; do not add compatibility semantics.
