# Rationalize Repository Quality Rules

## Why

The architecture gate mixed product invariants with repository-shape
preferences. Package ownership and dependency direction protect maintainable
boundaries; fixed file, directory, function, complexity, and nesting ceilings
do not prove those properties. Treating both categories as merge vetoes created
false precision, encouraged mechanical rewrites, and made the checker itself a
large maintenance owner.

## What Changes

- Keep positive package topology, dependency direction, composition roots,
  semantic naming, portability, documentation, and public-surface contracts as
  blocking invariants.
- Remove flat-directory, suffix-group, source-size, function-size, complexity,
  nesting, and path-ratchet vetoes and their private measurement code.
- Admit a future numeric veto only when its risk model, measurement semantics,
  false-positive cost, remediation path, and review trigger are explicit.
- Preserve the existing behavior, static-analysis, and coverage proof floors.

## Boundaries

This change does not alter AIGW product behavior, provider or client support,
credential handling, release identity, Forge topology, or runtime projection.
It adds no compatibility layer and no second quality authority.
