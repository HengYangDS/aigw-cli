## Decision

Use the `main` push from the repository's atomic maintainer publication as the
single accepted-publication event. That pipeline owns both the complete quality
graph and the bounded assertion that peer `main` and `dev` name the event's
exact commit.

Developer delivery remains review-driven: a pull request or merge request into
`dev` verifies its exact head before merge. The resulting `dev` push owns no
additional evidence and therefore starts no pipeline. This preserves one
verification graph per lifecycle stage while ensuring ordinary proposal merges
are never interpreted as already accepted releases.

The CUE model remains the only CI topology authority. GitHub Actions and GitLab
CI are generated projections of the same lifecycle model.
