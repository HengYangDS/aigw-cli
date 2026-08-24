## Decisions

### Test the portable transaction separately from the POSIX representation

The existing cross-platform test continues to prove creation, projection,
withdrawal, and preservation of later user content. A build-constrained Unix
test owns the additional `0600` assertion. This follows the same capability
boundary already used by the model-catalog permission contract.

### Verify `dev` by object identity, not duplicate execution

The complete graph remains owned by the `main` push for a maintainer atomic
publication and by the review SHA for a developer proposal. A `dev` push starts
one lightweight job which fetches peer `main` and requires both refs to resolve
to `CI_COMMIT_SHA` or `github.sha`.

The check proves only accepted-ref parity. It does not claim a second native,
source, release, or runtime execution.

### Separate Markdown parsing, presentation, and navigation concerns

Prettier owns deterministic Markdown presentation and markdownlint owns
parser-visible semantic conventions. The repository architecture gate owns the
two concerns neither tool can prove alone: a delimiter-shaped row must agree
with its preceding header before it can silently fall outside GFM table parsing,
and every canonical product document must be reachable from a declared reader
entrypoint. OpenSpec archives remain immutable historical records and are not
rewritten by the current-document formatter.
