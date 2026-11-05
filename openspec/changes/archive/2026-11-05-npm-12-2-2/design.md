## Context

aigw locks npm itself as a Mise tool (`npm = "npm:npm"`), so its repository
tools do not depend on the npm that Node bundles. Mise records the locked npm's
own dependency graph in an aube sidecar under `.config/mise/locks/npm/`.

## Goals / Non-Goals

**Goals:**

- Remove the six advisories that npm 12.2.2 fixes from the locked npm.

**Non-Goals:**

- Overriding npm's bundled dependencies; npm publishes them as one bundle.

## Decisions

### Move npm without the minimum release age

The supply-chain specification moves a locked tool to its first fixed release
when an advisory names it. The locked npm bundles undici 6.29.0 and ip-address
10.7.2, which six advisories name, and 12.2.2 is the first npm release that
bundles fixed versions of both.

### Regenerate the lock with Mise

`mise lock` writes the new version, its checksum and its aube graph, and prunes
the 12.2.1 entry, so no hand-written lock content is introduced.

## Risks / Trade-offs

- glob 13.0.6 and minimatch 10.2.5 in the npm bundle still have one advisory
  each; no npm release fixes them yet.
