# Product quality delta

## ADDED Requirements

### Requirement: one complete quality graph

The repository SHALL expose one executable source-quality graph reused by
local development, CI, and exact-HEAD governance proof.

#### Scenario: a new package or test owner is added

- **WHEN** tracked Go source changes
- **THEN** architecture, static analysis, formatting, coverage, governance, and
  cross-platform contracts evaluate the new owner without an exclusion list.

### Requirement: portable repository text

The repository SHALL define editor intent and Git checkout normalization using
tracked, platform-neutral configuration.

#### Scenario: a contributor uses another operating system

- **WHEN** Git checks out tracked text
- **THEN** line endings and executable semantics remain deterministic.

### Requirement: actor-independent contribution policy

The source SHALL require structured, signed commits without binding a personal
name, email, key, fingerprint, host path, or Forge credential.

#### Scenario: an admitted team contributor commits

- **WHEN** a commit enters protected history
- **THEN** its message and signature satisfy policy using external trust input.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:one complete quality graph` | `1.1` | `mise exec --locked -- go run ./tools/ci source` |
| `product-quality:portable repository text` | `1.2` | `mise exec --locked -- go run ./tools/repository --root . governance` |
| `product-quality:actor-independent contribution policy` | `1.3` | `ethos prove --execute --expect-head HEAD --json` |
