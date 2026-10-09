# AIGW Architecture Edition Provider

This directory is the sole AIGW-owned input boundary for Architecture
Publisher. It describes one selected AIGW architecture as portable data; it
does not add an AIGW command, execute product code, publish artifacts, or grant
acceptance.

## Owned values

| Path                      | Responsibility                                                                         |
| ------------------------- | -------------------------------------------------------------------------------------- |
| `provider.json`           | Closed `architecture.edition-provider/v1` selection.                                   |
| `selection.json`          | Exact Publisher package and migration-parity identities selected by AIGW.              |
| `_source/manifest.json`   | Source Bundle manifest for the selected AIGW revision.                                 |
| `_source/semantic.json`   | Current Claim Model and semantic source of the Edition.                                |
| `_source/source/**`       | Immutable source bytes named by the Claim Model provenance.                            |
| `edition.json`            | Audience, questions, coverage, narrative, and medium-independent visual grammar.       |
| `evolution.json`          | Declared comparison with the retained registry-era architecture.                       |
| `provider-evolution.json` | Deterministic inline projection of `evolution.json` consumed by the Provider contract. |
| `history/**`              | Immutable values required to reproduce that comparison.                                |

The `_source` prefix prevents Go tooling from treating captured provenance as
live packages. The files below it are a closed Source Bundle, not a second
implementation. `provider-evolution.json` is generated from the path-based
comparison and has no independent authority. Live AIGW behavior remains owned
by the repository source paths named in the Claim Model.

## Lifecycle

1. Select an exact AIGW revision and update the Claim Model and Edition when
   their intended meaning changes.
2. Use the exact published Publisher package named by `selection.json`. Supply
   an explicit `architecture.pack-input/v1` document and its byte digest to
   `source pack <pack-input> --sha256 <digest>` for Source Bundle construction.
   `edition build` consumes the declared Provider and produces reproducible
   output; AIGW does not add another materialization controller.
3. Review the declared Provider inputs and their exact identities together
   through their AIGW owners. Preserve captured sources and comparison history
   until their consumers release them; no automatic live-source update is
   provided.
4. Run the source contract test, then the installed-provider test against the
   exact package archive selected by `selection.json`.
5. Treat generated Candidate, PNG, SVG, HTML, qualification, and publication
   bytes as reproducible outputs. Do not commit them here as source truth.
6. Admit and publish those outputs only through AIGW's own review, evidence,
   release, and custody lifecycle.

The installed-provider test performs two native builds in isolated directories,
compares their complete results and verifies the selected input and output
identities. The source test verifies the declared Provider, Source Bundle and
comparison. Neither test rewrites the tracked inputs.

Architecture Publisher is a non-authorizing compiler. A successful build or
conformance run cannot advance AIGW refs, approve an AIGW Change, or replace
AIGW's architecture and quality policies.
