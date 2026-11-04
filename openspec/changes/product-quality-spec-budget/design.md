## Context

The repository caps each main specification at 500 non-blank lines (`.config/checks/scc/policy.toml`). While a Change is active, its delta stands in for the specification it rewrites, so `verification-time-bounds` passed the quality gate until its archive merged the requirement into `product-quality`. The merged specification has 512 non-blank lines, and the quality job refused it.

## Goals / Non-Goals

**Goals:**

- Keep the hang-bound obligation and all four of its scenarios unchanged in meaning.
- Bring `product-quality` within its budget in the same lane, before anything lands.

**Non-Goals:**

- Splitting `product-quality` into separate capabilities.
- Changing any other requirement.

## Decisions

### Restate the new requirement instead of splitting the specification

The requirement was written with wrapped lines and repeated context. One line per clause keeps the same obligation and scenarios in 14 non-blank lines, so the merged specification has 497. A split by cohesive concern is the durable answer, but it moves many unrelated requirements, and it belongs to the next requirement that `product-quality` gains.

### Use a second Change in the same lane

The archive of `verification-time-bounds` is already published on the proposal ref, so undoing it is refused. A MODIFIED delta in its own Change is the native way to rewrite a main specification. Both Changes land together.

## Risks / Trade-offs

- With 497 of 500 lines, `product-quality` has almost no room left; the next requirement needs the split first.
