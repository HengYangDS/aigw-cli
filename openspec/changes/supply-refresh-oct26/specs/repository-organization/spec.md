## ADDED Requirements

### Requirement: Generated output stays outside the Go source graph

The Go module SHALL declare its generated-output root as outside its source
graph. Package patterns, static analysis, tests and coverage SHALL then match
only product and tool source in every checkout, including a canonical checkout
that retains evidence.

#### Scenario: Retained evidence contains Go source

- **WHEN** the ignored generated-output tree holds captured Go files from
  different packages
- **THEN** `go vet ./...`, `go list ./...` and the repository's quality graph
  neither load nor report them
- **AND** no evidence file has to be moved, renamed or deleted for a proof to
  pass.
