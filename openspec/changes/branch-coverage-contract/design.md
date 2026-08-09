# Design

## Decision

Do not adopt `github.com/rillig/gobco` or build a repository-owned branch
instrumenter.

The evaluated stable release accepts one instrumented package per process. A
module result therefore repeats the test suite for every production package,
loses normal single-run semantics, and scales with package count. The available
multi-package fork is untagged source-rewriting infrastructure and fails valid
platform-specific test files. Owning an AST or `-toolexec` instrumenter locally
would violate the deletion-first build-vs-buy rule.

## Contract

| Measure | Authority | Threshold |
| --- | --- | --- |
| Aggregate statements | Go atomic cover profile | `> 95%` |
| Per-package statements | Same profile, grouped by package | `> 95%` |
| Branches | No admitted authority | Not claimed |

The statement gate executes `go test` exactly once for `./...` with
`-coverpkg=./...`. Black-box and acceptance tests therefore contribute to the
source packages they exercise, after which one profile is grouped by package.
This preserves legitimate integration evidence without duplicating white-box
tests.

## Reuse and exclusions

No custom AST rewriter, compatibility wrapper, parallel script, fork, or
runtime dependency is introduced. A future branch measure requires one stable,
maintained tool that covers the complete module in one execution on every
supported platform.
