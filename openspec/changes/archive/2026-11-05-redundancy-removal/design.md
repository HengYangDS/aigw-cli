## Context

The repository already gives ETHOS branch and tag publication, and
`docs/architecture/repository-tooling.md` rules out a shared test-support
API. The review traced every removal to its callers on darwin, linux and
windows, including CI projections, documentation and specifications.

## Goals / Non-Goals

**Goals:**

- One owner for each job, and no code without a consumer.

**Non-Goals:**

- Behavior changes to the released programs.
- Refactors that change how a product owner works, which need their own
  review.

## Decisions

### Retire the publication commands rather than wrap ETHOS

`ethos publish` already pushes atomically with a lease and records receipts;
the operations guide names it for branches and tags.

### Move duplicated tests to the subject's owner

A cross-command readiness test belongs with the command that owns readiness.
Moving tests keeps one fixture without a new shared package.

## Risks / Trade-offs

- A removed subcommand could have an unrecorded operator use; the operations
  guide names its replacement.
