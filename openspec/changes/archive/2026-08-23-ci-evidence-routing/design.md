## Decision

The CUE authority defines four verification entry routes:

| Lifecycle event | GitHub | GitLab |
|---|---|---|
| Developer review into `dev` | pull request | merge request pipeline |
| Maintainer accepted publication | `main` push | default-branch push |
| Release | tag release workflow | tag pipeline |
| Explicit diagnosis | workflow dispatch | web/API pipeline |

Proposal pushes, `dev` pushes, and review-plus-push pairs do not create parallel
proof. Both Forge projections retain the same logical jobs; only their native
trigger syntax differs. A review validates its immutable head SHA. Accepted
publication validates the exact signed product object at `main`, while `dev`
remains an identical distribution ref.
