## Decision

The public `release build` command reads `VERSION`, resolves the matching
`CHANGELOG.md` heading, and accepts only the output directory. Tag CI validates
the tag against that same version and uses the same resolver. A missing heading
fails closed in every environment.

This removes environment-dependent release bytes without adding another
configuration carrier. `VERSION` remains the version SSOT and `CHANGELOG.md`
remains the chronology and release-time SSOT.
