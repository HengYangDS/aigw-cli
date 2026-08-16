# Complete Rule Rationality

## Why

The remaining architecture gate still mixed positive repository contracts with
language bans, text blacklists, and syntax heuristics. Those checks encoded the
current implementation rather than a durable product risk and made legitimate
future tools, hosts, and contributors fail for unrelated reasons.

## What Changes

- Keep one declarative package topology, dependency direction, composition-root
  boundary, semantic carrier grammar, Decision Record contract, and deterministic
  text-byte contract.
- Derive Go import identity from `go.mod`; do not repeat the product module path
  in checker code.
- Remove unconditional bans on Python, shell carriers, private addresses,
  personal-path text, aliases, and forwarding wrappers.
- Remove English-only and retired-directory-name vetoes; language and directory
  labels alone are not product-quality evidence.
- Require the architecture policy to state its risk model, measurement,
  false-positive cost, remediation, and review condition.
- Remove Decision Record sequence contiguity as a historical ratchet; uniqueness,
  semantic names, registration, and required content remain authoritative.
- Replace strict per-package coverage thresholds with one aggregate product-risk
  threshold, mandatory per-package execution evidence, and exact diagnostic
  package ratios.

## Boundaries

This change does not alter AIGW runtime behavior, client support, provider
support, credentials, releases, or Forge topology. It adds no compatibility
surface and no second policy owner.
