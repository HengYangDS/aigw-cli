## ADDED Requirements

### Requirement: Independent Forge parity

GitLab and GitHub SHALL remain independent publication planes. A provider-specific identity projection SHALL prove exact accepted tip-tree parity and independently verify the complete target commit provenance and release-tag trust. Deterministic collapse of semantically duplicate source commits SHALL NOT be treated as source drift.

#### Scenario: Equivalent provider projection

- **WHEN** provider identity normalization maps duplicate semantic commits or parents to the same target object
- **THEN** the projected branch is accepted only when its tip tree exactly equals the canonical accepted tip tree and all provider-native provenance checks pass

#### Scenario: Real source drift

- **WHEN** a projected branch tip resolves to a different tree
- **THEN** synchronization fails before publication

### Requirement: Portable exact-version CI bootstrap

The GitLab Linux bootstrap SHALL derive its mise version from the repository lock authority and SHALL use bounded, retryable HTTP transport without querying a foreign Forge API.

#### Scenario: Transient HTTP transport failure

- **WHEN** an installer or asset transfer encounters a transient transport error
- **THEN** the bootstrap retries a bounded number of times over HTTP/1.1 and still verifies the upstream release checksum before installation

## Requirement To Task To Proof

| Requirement | Task | Proof |
|---|---|---|
| `product-quality:Independent Forge parity` | `1.4` | `go-test-tools-forge-and-forge-sync` |
| `product-quality:Portable exact-version CI bootstrap` | `2.3` | `go-test-tools-ci-and-generated-projection-parity` |
