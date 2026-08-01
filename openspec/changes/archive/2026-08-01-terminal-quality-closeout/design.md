## Context

The product-convergence change already owns AIGW's terminal architecture. This
change closes two concrete enforcement gaps without creating a second design
authority.

## Decisions

### Package contracts are checked at the semantic owner

The architecture checker parses production package comments and requires one
accurate `Package <name>` contract per non-`main` package. The real policy
enables the rule; generic checker fixtures keep it off unless the rule itself
is under test, preserving orthogonal tests.

The comments live in the primary implementation files. Separate `doc.go`
carriers were rejected because four one-line carriers add entities and files
without adding behavior or ownership clarity.

### Handled errors remain quiet

The synchronization failure test captures both command output streams and
rejects framework usage, warnings, tracebacks, and false completion output.
This is a behavioral assertion, not a log-string allowlist for hosted CI.

### Toolchain projections derive from one source

`go.mod` owns the exact release toolchain. CI and README project that version;
contract tests read `go.mod` instead of repeating a second expected version.
The release toolchain gate compares the active compiler to the same source and
fails before any expensive validation when the host or projection drifts.

## Risks / Trade-offs

- A stale package comment can still be semantically weak. Review and accurate
  wording remain necessary, while the gate prevents complete omission or a
  mismatched package name.
- The focused CLI test does not prove every command path. Root-command
  acceptance tests continue to own the broader terminal grammar.
- Exact release toolchains require an intentional `go.mod` update when the
  managed stable compiler advances; projections must change atomically.

## Closeout

Run the focused RED/GREEN tests, then freeze the complete candidate and run the
single full coverage and declarative verification boundary. Archive this
change before signed commit and landing.
