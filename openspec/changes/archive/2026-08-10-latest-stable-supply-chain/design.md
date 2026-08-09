## Context

An isolated clone at the exact accepted candidate HEAD showed one resolver
delta: `github.com/charmbracelet/ultraviolet` advances to its latest available
stable pseudo-version. The complete Go test suite passed with that graph.

## Decision

Apply the resolver output without manually promoting transitive modules to
direct dependencies or changing unrelated versions. Remove the archive
projection's surplus terminal newline rather than weakening the architecture
gate or adding another formatter.

## Verification

Run the complete native repository gates and exact-HEAD ETHOS proof.
