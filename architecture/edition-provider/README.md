# AIGW Architecture Edition Provider

This directory is the sole AIGW-owned input boundary for Architecture
Publisher. It describes one selected AIGW architecture as portable data; it
does not add an AIGW command, execute product code, publish artifacts, or grant
acceptance.

## Owned values

| Path                      | Responsibility                                                                                                     |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `provider.json`           | Closed `architecture.edition-provider/v1` selection.                                                               |
| `selection.json`          | Exact Publisher package and migration-parity identities selected by AIGW.                                          |
| `_source/manifest.json`   | Source Bundle manifest for the selected AIGW revision.                                                             |
| `_source/semantic.json`   | Current Claim Model and semantic source of the Edition.                                                            |
| `_source/source/**`       | Immutable source bytes named by the Claim Model provenance.                                                        |
| `edition.json`            | Audience, questions, coverage, narrative, and medium-independent visual grammar.                                   |
| `evolution.json`          | Declared comparison with the retained registry-era architecture.                                                   |
| `provider-evolution.json` | Deterministic inline projection of `evolution.json` consumed by the Provider contract.                             |
| `history/**`              | Immutable values required to reproduce that comparison.                                                            |
| `materialize.mjs`         | Maintainer-only deterministic materialization from live selected source and one exact published Publisher package. |

The `_source` prefix prevents Go tooling from treating captured provenance as
live packages. The files below it are a closed Source Bundle, not a second
implementation. `provider-evolution.json` is generated from the path-based
comparison and has no independent authority. Live AIGW behavior remains owned
by the repository source paths named in the Claim Model.

## Lifecycle

1. Select an exact AIGW revision and update the Claim Model and Edition when
   their intended meaning changes.
2. Run `materialize.mjs` with the exact archive, package identities, and release
   manifest named by `selection.json`. It derives the source inventory from the
   Claim Model and invokes the selected Publisher's native `source pack` and
   `edition build` operations.
3. Update `provider.json` and `selection.json` with the resulting exact
   identities.
4. Run the source contract test, then the installed-provider test against the
   exact package archive selected by `selection.json`.
5. Treat generated Candidate, PNG, SVG, HTML, qualification, and publication
   bytes as reproducible outputs. Do not commit them here as source truth.
6. Admit and publish those outputs only through AIGW's own review, evidence,
   release, and custody lifecycle.

Use `--check` to reproduce the complete generated state without changing tracked
files. Materialization validates every result before replacing any tracked file;
each replacement is atomic and rollback restores the prior set if a write fails.

Architecture Publisher is a non-authorizing compiler. A successful build or
conformance run cannot advance AIGW refs, approve an AIGW Change, or replace
AIGW's architecture and quality policies.
