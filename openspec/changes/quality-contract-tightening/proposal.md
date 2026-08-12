## Why

AIGW has repository-owned coverage, architecture, CI, and release policies, but
their admitted scope and quantitative claims are not yet one complete contract.
In particular, statement coverage alone cannot support a branch-coverage claim,
and source-only structure checks can leave tests, repository tools, documents,
packaging, installation, and runtime evidence outside the enforced boundary.

## What Changes

- **BREAKING** Replace partial or ambiguous quality claims with one positive,
  machine-readable contract covering product source, tests, repository tools,
  CI, documentation, OpenSpec, build, release, installation, and runtime
  evidence.
- Require independently measured statement and branch coverage, plus package
  completeness, each strictly greater than 95 percent for every Go package and
  for the whole module.
- Apply semantic ownership, dependency direction, size, complexity, naming,
  portability, documentation, and command-contract checks to every applicable
  owner, including test and tool code, without an exclusion or compatibility
  surface.
- Make local verification, ETHOS proof, GitLab CI, and GitHub Actions consume
  the same repository policy while retaining independent execution and
  publication failure domains.
- Remove superseded thresholds, duplicate command bodies, misleading evidence,
  and residual compatibility paths once the unified contract passes.

## Authority Boundary

The repository owns product and quality policy. ETHOS governs lifecycle and
exact-HEAD evidence but does not become an AIGW runtime dependency. CI providers
project repository commands; they do not define them. Contributor identity,
signing trust, Forge coordinates, credentials, host paths, external gateways,
workstation products, IDE state, and Codex conversation data remain external
inputs or out of scope.

## Reuse Stance

Use Go-native analysis and maintained tools where they express the required
measure faithfully. Add repository code only for AIGW-specific composition or a
contract that no admitted tool provides. One policy owner may have several
platform projections; no projection becomes a second policy owner.

## Capabilities

### Modified Capabilities

- `product-quality`: one comprehensive, quantitative, portable quality graph.

## Non-goals

- Changing AIGW's provider-neutral control-plane product boundary.
- Managing Proxy, Workstation, JetBrains, Codex history, or client lifecycle.
- Coupling GitLab availability to GitHub, or GitHub availability to GitLab.
- Preserving obsolete quality commands, thresholds, wrappers, or evidence.

## Impact

This change may reorganize production, test, and repository-tool packages;
tighten repository and CI policy; and update documentation and release proof.
It intentionally permits destructive cleanup, but does not add product
behavior or retain source-level compatibility aliases.
