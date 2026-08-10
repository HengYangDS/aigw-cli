# ci-diagnostics Specification

## Purpose

Define quiet, deterministic hosted CI diagnostics that expose actionable
failures without relying on runner-global state.

## Requirements
### Requirement: Hosted Git initialization is explicit

GitHub verification and release workflows SHALL declare `main` as Git's
process-scoped default branch without changing runner-global configuration.

#### Scenario: A hosted action initializes a repository

- **WHEN** checkout or a test fixture initializes Git state
- **THEN** Git resolves `main` as the default branch
- **AND** the run emits no default-branch initialization hint
- **AND** no verification or provenance gate is weakened
