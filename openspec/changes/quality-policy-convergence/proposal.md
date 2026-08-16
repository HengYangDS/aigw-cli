# Converge Quality Policy

## Why

AIGW still mixed product-risk controls with presentation preferences and
repeated part of the coverage contract in prose. That made a hand-written
whitespace checker a merge authority and left quantitative policy without an
explicit false-positive and review model.

## What Changes

- Keep deterministic byte invariants, semantic ownership, dependency direction,
  behavior, portability, security, and evidence authenticity as merge gates.
- Remove blank-run, trailing-blank-line, and TOML/INI table-spacing vetoes from
  the repository checker; language formatters and review own presentation.
- Make the coverage policy the sole owner of its floor, comparison, risk model,
  measurement, false-positive cost, remediation, and review condition.
- Replace obsolete negative owner-shape language with positive topology and
  dependency contracts.

## Boundaries

This change does not alter AIGW runtime behavior, providers, clients,
credentials, release identity, CI topology, or supported platforms. It adds no
compatibility parser and no second policy registry.
