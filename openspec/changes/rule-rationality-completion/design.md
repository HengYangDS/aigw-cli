# Design

## Admission Principle

A blocking repository rule must protect an observable semantic risk through one
satisfiable contract. The architecture policy therefore owns both the positive
topology and the rationale for enforcing it.

| Retained contract | Protected risk | Measurement |
| --- | --- | --- |
| Package topology and import direction | Hidden coupling and authority drift | Parsed Go packages and imports against the declared graph |
| Composition roots and peer boundaries | Behavior accumulating in assemblers | Production files and imports against declared owners |
| Semantic carrier names and Decision Records | Ambiguous discovery and duplicate decisions | Tracked carrier grammar and canonical register |
| Deterministic text bytes | Cross-host diff and checkout drift | Encoding-independent LF, final newline, trailing whitespace |
| Aggregate coverage and package observation | Broad unobserved behavior or a wholly unexecuted owner | Exact aggregate counts plus non-zero execution evidence and ratios for every package |

## Removed Negative Surfaces

The checker no longer infers architecture from the presence of a programming
language, shell, address literal, user path, alias, or small forwarding
function. The aggregate governance gate likewise no longer rejects non-English
text, historical directory labels, or a missing fixed-form package comment.
These signals are either valid in bounded contexts or already covered by the
compiler, release validation, documentation review, or the positive dependency
graph.

## Single Sources

- `go.mod` owns the module identity.
- `.config/checks/architecture/policy.toml` owns the semantic graph and its
  enforcement rationale.
- `tools/architecture` interprets those owners without product-name literals.
- OpenSpec describes observable policy, not a second executable threshold.

## Quantitative Boundary

Coverage is evidence, not a proxy for local design quality. The aggregate
statement and branch floors limit the product's total unobserved behavior.
Every production package must still appear in the evidence, execute at least
one statement, and execute at least one branch when it owns branches. Exact
package ratios remain visible for review, but a small denominator cannot reject
an otherwise proved change by itself.
