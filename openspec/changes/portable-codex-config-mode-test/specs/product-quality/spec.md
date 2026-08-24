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

### Requirement: canonical Markdown has executable structure and navigation proof

Current product Markdown SHALL be formatted and linted by locked tools. The
repository gate SHALL reject delimiter-shaped table rows whose column count does
not match the preceding header, even when a Markdown parser would otherwise
treat the lines as ordinary prose. Every canonical product document SHALL be
reachable from a declared reader entrypoint. Immutable OpenSpec archives SHALL
remain outside current-document rewriting.

#### Scenario: a malformed table bypasses parser-based formatting

- **WHEN** a Markdown header and its delimiter row declare different column counts
- **THEN** repository verification SHALL reject the file before publication
- **AND** the diagnostic SHALL identify the file and delimiter row.

#### Scenario: a canonical document loses navigation

- **WHEN** a canonical product document is no longer reachable from any declared entrypoint
- **THEN** repository verification SHALL reject the orphan document
- **AND** external lifecycle carriers SHALL remain governed by their own entrypoints.
