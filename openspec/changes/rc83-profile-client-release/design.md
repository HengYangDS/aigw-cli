# Design

`VERSION` remains the release-identity authority; `CHANGELOG.md` describes the
user-visible delta. GitLab and GitHub each receive their own signed history,
tag, CI, assets, and Release from the same product tree. Neither Forge depends
on the other.

The release is blocked unless native macOS, Linux, and Windows source
acceptance pass. A runner-host failure remains infrastructure evidence and is
not converted into a product workaround or cross-platform substitute.
