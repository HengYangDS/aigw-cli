# Design

## Decision

Use `github.com/rillig/gobco` in branch mode. It instruments controlling boolean
conditions and switch branches without adding product runtime dependencies. The
repository invokes it package by package because the tool deliberately accepts
one package per process. Production packages come from the same `go list ./...`
set already used by the statement gate.

## Contract

| Measure | Authority | Threshold |
| --- | --- | --- |
| Aggregate statements | Go atomic cover profile | `> 95%` |
| Per-package statements | Same profile, grouped by package | `> 95%` |
| Aggregate branches | Summed `gobco -branch` counters | `> 95%` |

The gate parses a stable final summary and fails closed on absent, malformed, or
contradictory branch output. Package enumeration, tool execution, and threshold
policy remain inside `tools/coveragegate` rather than CI shell logic.

## Reuse and exclusions

`gobco` is build tooling only and is locked as a Go tool dependency. No custom
AST rewriter, compatibility wrapper, parallel script, or runtime dependency is
introduced. Its documented lack of `select` instrumentation is explicit; this
change does not misstate condition coverage as complete control-flow coverage.
