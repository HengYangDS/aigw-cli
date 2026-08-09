# Design

## Decision

Use one readable noun for each package owner and one command plane for each
repository responsibility. Enforce the direct owner set and dependency
direction declaratively instead of preserving a list of historical forbidden
names.

## Semantic owners

| Current surface | Terminal owner | Reason |
| --- | --- | --- |
| `internal/upgrade` | `internal/upgrade` | The product operation is an upgrade, not a package acting on itself |
| `tools/release` | `tools/release` | Build, admission, asset verification, and publication are one release tool |
| `tools/coverage` | `tools/coverage` | The command owns coverage proof; `gate` is behavior, not a domain |
| `tools/repository` | `tools/repository` | Repository conformance is one explicit command surface |
| CI contract and execution commands | `tools/ci` | CI projection validation and execution share one composition root |

Repository release validation and product upgrade validation remain separate
because their inputs and authority boundaries differ. Repository tooling must
not reach into `internal/upgrade` merely to reuse implementation. Package
movement is a hard cutover: no forwarding directories, type aliases,
deprecated commands, or parallel invocation paths remain.

## Dependency direction

```text
tools/ci ──> tools commands (process invocation only)
tools/release ──> repository release inputs
internal/upgrade ──> product release sources
cmd/aigw ──> internal/cli ──> product domains
```

The architecture gate declares the direct semantic owners under `cmd`,
`internal`, and `tools`, then reads imports to reject undeclared edges. This is
a positive topology contract: a new domain is admitted explicitly, while new
provider implementations remain open beneath `internal/providers`. The
package graph remains source truth rather than a parallel hand-written model.

## Rejected alternatives

| Alternative | Rejection |
| --- | --- |
| Preserve old command paths as wrappers | Creates parallel semantics and permanent maintenance cost |
| Use hyphenated variants such as `release-kit` | Improves typography but preserves the vague entity |
| Enumerate retired names in governance code | Encodes history instead of defining the admitted topology |
