## Context

See `proposal.md`. The existing candidate already contains the behavioral
rewrite; this Change binds that implementation to one product contract and the
ETHOS lifecycle without creating a parallel plan.

## Goals / Non-Goals

**Goals:**

- Make configuration and projection the complete AIGW authority boundary.
- Make package placement follow the behavior's reason to change.
- Keep provider and publication extension points explicit and narrow.
- Fail closed on portability, ownership, coverage, provenance, and projection
  integrity.

**Non-Goals:**

- Owning a Responses proxy, provider wire adaptation, IDE runtime, or session.
- Publishing AIGW as an importable public Go library.
- Preserving obsolete package names, wrappers, or implicit provider defaults.

## Decisions

### Configuration is the product kernel

`internal/configuration` owns the Account, Profile, Route, Adapter, endpoint,
storage-policy, persistence, and token-free manifest model. Secrets, generic
credential validation, native provider diagnostics, and client projections are
leaf capabilities around that kernel. This replaces generic `domain`, `config`,
and command buckets whose reasons to change overlapped.

Alternative: a registry type for every provider. Rejected because ordinary
providers differ only in data and endpoint selection; only native diagnostics
justify a leaf implementation.

### Composition does not imply ownership

An external Responses service is selected through a normal Account endpoint.
Neither product imports the other or manages the other's configuration,
deployment, service, or state.

Alternative: AIGW-specific proxy setup and health commands. Rejected because
they couple independent products and force every new gateway through AIGW.

### Projection is guarded and transactional

Planning, validation, commit, and compensation have distinct owners. Marked
artifacts and sidecars establish write authority; byte-exact preimages and
postimages prevent compensation from overwriting a concurrent newer writer.

Alternative: best-effort sequential writes. Rejected because partial client
configuration is not a valid steady state.

### Product identity and publication identity are separate

The local `aigw-cli` Go module is a non-fetchable build identity for an
executable product. Forge coordinates, contributor identities, signing keys,
and trust stores enter only through protected publication context.

Alternative: encode a private Forge module path or contributor in source.
Rejected because it converts deployment metadata into a portability dependency.

### Quality policy has machine owners

Native configuration files own coverage and architecture thresholds; reusable
scripts execute them; CI only projects those commands. The architecture gate
checks semantic composition roots, forbidden names and references, wrappers,
aliases, directory budgets, and dependency direction.

## Risks / Trade-offs

- **Breaking removal of old commands or package names** -> keep migration prose
  in current docs, not runtime compatibility layers.
- **Provider-native diagnostics still require code** -> limit them to an
  optional leaf registry; generic routing remains data-only.
- **Process-local compensation is not a cross-process transaction** -> guarded
  preimages and sidecar ownership fail closed rather than pretending to provide
  distributed locking.

## Migration Plan

1. Validate the staged candidate against the product contract and strict local
   gates.
2. Freeze and independently reconstruct the exact candidate tree.
3. Create Forge-specific trusted commits from the same source tree, run hosted
   CI, and compare publication outputs.
4. Land through ETHOS only after exact-HEAD proof; retain rollback evidence and
   remove superseded lanes only after owner-bound closeout checks.
