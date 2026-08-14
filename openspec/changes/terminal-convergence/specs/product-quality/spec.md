## MODIFIED Requirements

### Requirement: Coverage exceeds the product threshold

Every Go package under `./...` and the module aggregate SHALL have statement
and branch coverage strictly greater than 95%. Package omission, generated
substitution, and aggregate-only success SHALL not satisfy the gate.

#### Scenario: A package falls to 95 percent

- **WHEN** statement or branch coverage for any package or the aggregate is at or below 95%
- **THEN** quality verification fails
- **AND** the release candidate cannot land.

### Requirement: Quality is platform-complete

Formatting, vetting, static analysis, architecture, security, dependencies,
documentation links, tests, release, and native platform checks SHALL apply to
the complete declared source surface with warnings treated as failures.

#### Scenario: A hosted platform lacks an admitted runner

- **WHEN** a required macOS, Linux, or Windows native job cannot execute
- **THEN** CI reports an unavailable required gate rather than waiting indefinitely or succeeding
- **AND** no cross-compile result substitutes for the native check.

### Requirement: Supply-chain versions have one maintained authority

Go, tools, actions, and release dependencies SHALL use current stable versions
through one repository-owned lock or declaration for each ecosystem. CI SHALL
consume those declarations rather than duplicate version literals.

#### Scenario: A stable dependency advances

- **WHEN** the locked supply chain is refreshed
- **THEN** local development, GitLab, and GitHub resolve the same declared versions
- **AND** obsolete pins and compatibility fallbacks are removed.
