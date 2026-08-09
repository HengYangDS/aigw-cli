## Context

An isolated clone at the exact accepted candidate HEAD showed one resolver
delta: `github.com/charmbracelet/ultraviolet` advances to its latest available
stable pseudo-version. The complete Go test suite passed with that graph.

## Decision

Apply the resolver output without manually promoting transitive modules to
direct dependencies or changing unrelated versions.

## Verification

Run the complete native repository gates and exact-HEAD ETHOS proof.
