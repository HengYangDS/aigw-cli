# DR-0010: Scope CI Evidence to Product Lifecycle Stages

- Status: accepted
- Date: 2026-08-22

## Context

A single proposal commit could start both branch-push and review pipelines on
each Forge. Publishing the same accepted object to `dev` and `main` then
started two more equivalent verification graphs. The duplicated executions
increased latency and runner cost without measuring a different product object
or lifecycle claim.

## Decision

CI execution is routed by product lifecycle stage:

- developer proposals receive complete verification on review into `dev`;
- maintainer publication receives complete verification on accepted `main`;
- release tags use the release pipeline;
- explicit manual dispatch remains available for diagnosis.

Proposal pushes and the accepted `dev` mirror do not independently own
verification. The CUE CI model remains the sole topology authority and projects
these semantics into GitHub and GitLab syntax. Every job still checks out and
measures the exact product commit selected by its lifecycle event.

## Consequences

One product commit produces one complete verification graph per Forge and
lifecycle stage. Review updates continue to retrigger verification. Maintainers
can publish a locally accepted object without manufacturing a review event, and
the equal `dev` ref remains available without duplicating accepted evidence.

## Revisit Trigger

Revisit if either Forge gains a portable, immutable evidence-reuse primitive
that can bind one successful graph to another lifecycle stage without rerunning
or weakening any gate.
