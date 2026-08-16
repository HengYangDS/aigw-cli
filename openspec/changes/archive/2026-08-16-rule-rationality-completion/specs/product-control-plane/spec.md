## REMOVED Requirements

### Requirement: Documented package ownership

**Reason:** Requiring every Go package to begin with one fixed English comment
shape measures wording rather than semantic ownership. Package topology,
dependency direction, exported API documentation, tests, and user-facing
documentation already carry the durable ownership contract.

**Migration:** Retain package comments where they improve Go documentation.
Do not make one comment prefix or language an independent publication veto.

#### Scenario: package documentation is reviewed

- **WHEN** maintainers review the ownership and public contract of a package
- **THEN** they evaluate its declared topology, dependencies, API, tests, and
  relevant documentation
- **AND** omission of a fixed-form `Package <name>` sentence does not
  independently reject the change.
