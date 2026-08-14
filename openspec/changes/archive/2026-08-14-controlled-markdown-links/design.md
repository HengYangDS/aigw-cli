# Design

## Decision

Make `tools/ci` the single owner of link-check inputs. `git ls-files -z --
'*.md'` positively defines repository-controlled Markdown; Lychee receives the
result as arguments rather than expanding a filesystem glob.

This excludes private and untracked projections by construction without a
directory blacklist or shell-specific quoting.

## Verification

- a focused test with tracked, untracked, and `.git` Markdown;
- the complete `tools/ci` test package;
- the real link and static gates from a checkout containing ETHOS runtimes.
