# Client Projection Architecture Edition

This directory owns the source-bound architecture values for AIGW's client
projection control plane. It does not introduce a runtime dependency, generated
site, second architecture policy, or publication command into AIGW.

## Authored values

| File                                                                           | Responsibility                                                                                       |
| ------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| [`claim-model.json`](claim-model.json)                                         | Binds selected product concepts and relationships to exact AIGW source bytes.                        |
| [`edition.json`](edition.json)                                                 | Selects the maintainer questions, static overview, interactive views, and qualification obligations. |
| [`evolution.json`](evolution.json)                                             | Compares the retained registry-era architecture with the current projection architecture.            |
| [`history/registry-v1-claim-model.json`](history/registry-v1-claim-model.json) | Retains the prior Claim Model needed by the declared evolution comparison.                           |
| [`history/registry-v1-edition.json`](history/registry-v1-edition.json)         | Retains the matching prior Edition.                                                                  |

The Claim Model covers the CLI, configuration, credentials, discovery,
synchronization, the ordered Client registry, the Adapter contract, guarded
transactions, and the Codex, Claude Code, Claude Desktop, and Hermes
projections. Its scope is deliberately selected rather than an inferred claim
that every implementation file is represented.

## Authority boundary

AIGW owns these portable JSON values. Architecture Publisher validates and
compiles them when selected for architecture acceptance; it does not discover
AIGW semantics or become part of the installed AIGW product. The repository's
existing architecture policy remains the executable owner of package and import
direction. OpenSpec remains the owner of active change intent and progress.

Generated SVG, PNG, HTML, candidate bundles, and qualification output are not
tracked here. Acceptance rebuilds them from the exact Claim Model, Edition,
Source Bundle, and selected Architecture Publisher package. This keeps authored
meaning in Git without retaining reproducible output as repository ballast.

## Views

- **Overview:** the complete selected control plane from operator intent to
  guarded native client projections.
- **Authority:** configuration, credential, and discovery ownership.
- **Adapters:** one ordered registry and one complete Adapter contract across
  admitted clients.
- **Transaction:** the guarded mutation and compensation boundary.

The static and interactive media are independent compositions over the same
required claims. Neither medium is evidence for the other.
