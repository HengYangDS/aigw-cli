# Design

## Single Policy

`.config/checks/coverage/policy.toml` remains the sole executable authority.
The checker measures one complete profile and applies its declared comparison
to both scopes:

| Scope | Statement evidence | Branch evidence |
| --- | --- | --- |
| Aggregate | Complete module counts | Complete analyzer counts |
| Package | Every canonical package | Every canonical package with branchless packages reported as 100% |

The accepted comparison is `greater-than` with a floor of `95.0`; therefore an
exact 95-percent result fails. Package absence, zero execution, malformed raw
evidence, and duplicate evidence continue to fail independently.

## Coverage Closure

`tools/coverage` receives contract tests for its package-threshold decisions.
`tools/architecture` exercises existing portable failure boundaries through
the package's current deterministic seams. Tests must demonstrate observable
behavior; empty assertions, ignored source, generated exclusions, and coverage
annotations are not admitted.

## Review Condition

Package thresholds can be denominator-sensitive. That cost is recorded in the
machine policy, but it does not weaken current enforcement. Reconsideration
requires evidence from three legitimate changes blocked only by denominator
granularity and must update the same policy rather than add an exception.
