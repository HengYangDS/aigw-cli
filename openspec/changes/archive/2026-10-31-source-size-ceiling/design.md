## Context

ETHOS measures effective lines per file and refuses a rebind while any file is
above its ceiling. The repository's SCC gate measures code lines per file with
its own limit. Since alpha.23 both apply to Go.

## Goals / Non-Goals

**Goals:** one size ceiling for Go code and main specifications, and every Go
file within it, with each moved group of tests named by its concern.

**Non-Goals:** compressing code to meet the limit, changing any test's behavior,
or splitting product code that is already within the limit.

## Decisions

- The SCC limits move to 500 rather than ETHOS gaining an exception: an
  exception above 500 needs a reason, and these files have none.
- Each file gives up the tests of one concern, so the files that remain read as
  one subject: endpoint validation from the domain model tests, retained
  credential fixtures from the client credential journeys, signing from the
  native construction tests, route purposes from the route acceptance tests,
  artifact trust from the release command tests, dependency scanning into the
  scanner tests that already own it, coverage profile cleanup from the coverage
  command tests, attribution from the performance journeys, build identity from
  the build tests, and the journey environment from the journey fixture.
- Moved declarations keep their bodies; only the import blocks change.

## Risks / Trade-offs

- A file with a build constraint moves its tests to a file with the same
  constraint, or they would stop running under that tag.
- Later tests that would push a file above 500 lines need their own concern's
  file; the gates refuse the alternative.
