## Decision

Create the next SemVer prerelease, rc.91, from the current accepted product
tree. Keep one release-version authority in `VERSION`; describe the complete
user-visible delta since rc.90 once in `CHANGELOG.md`.

The source commit and annotated tag are constructed and signed locally, then
projected unchanged to GitLab and GitHub. Publication and installed-runtime
acceptance remain separate proof surfaces.
