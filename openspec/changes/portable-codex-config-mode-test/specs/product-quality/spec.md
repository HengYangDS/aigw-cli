## ADDED Requirements

### Requirement: accepted ref parity is visible without duplicate proof

When a maintainer publication atomically advances peer `main` and `dev` to one
accepted product object, each peer SHALL expose a bounded `dev` result that
proves both protected refs resolve to the event's exact commit. The `main`
event SHALL remain the sole owner of the complete verification graph for that
publication. The `dev` result SHALL NOT repeat source, native, package, release,
or runtime verification.

#### Scenario: a peer receives one accepted object on main and dev

- **WHEN** an atomic maintainer publication advances peer `main` and `dev`
- **THEN** the `main` event SHALL run the complete verification graph
- **AND** the `dev` event SHALL run one exact-ref parity check
- **AND** that check SHALL require both protected refs to equal the event commit
- **AND** no complete verification job SHALL run again for the `dev` event.

### Requirement: repository text quality has one mature owner per concern

Portable byte invariants SHALL be declared in `.editorconfig` and verified by a
locked cross-platform EditorConfig implementation. Current product Markdown
SHALL be formatted by Prettier, linted by markdownlint, and checked for explicit
link validity by lychee. Repository-specific analyzers SHALL NOT duplicate
those responsibilities or infer links that an author did not declare.
Immutable OpenSpec archives SHALL remain outside current-document rewriting.

#### Scenario: a current text artifact violates its declared contract

- **WHEN** a tracked text file violates `.editorconfig` or a current Markdown
  file violates the locked formatter, linter, or explicit-link contract
- **THEN** repository verification SHALL reject the exact artifact
- **AND** the responsible mature tool SHALL emit the diagnostic.
