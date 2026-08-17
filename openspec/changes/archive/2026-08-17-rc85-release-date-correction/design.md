# Design

## Decision

Treat the Changelog heading as release metadata owned by the release chronicle.
Because rc.85 is not yet published, correct the heading before either Forge
creates its provider-native tag. Do not backdate tagger metadata or rewrite any
published history.

## Verification

1. Validate the OpenSpec Change and Changelog structure.
2. Prove the exact corrected source head.
3. Land and close out the Change through the repository lifecycle.
4. Refresh GitLab and GitHub projections before creating either rc.85 tag.
