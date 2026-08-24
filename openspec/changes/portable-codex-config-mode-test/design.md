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

### Give each text-quality concern one mature owner

EditorConfig Checker owns portable byte-level invariants from `.editorconfig`.
Prettier owns deterministic current-Markdown presentation, markdownlint owns
Markdown structure and conventions, and lychee owns explicit-link validity.
The curated documentation index remains the navigation authority instead of a
second crawler attempting to infer missing author intent. Repository-specific
checks remain reserved for product rules no maintained tool can express.
OpenSpec archives are immutable historical records and are not rewritten by the
current-document formatter.
