# Design

`VERSION` remains the sole release-identity authority; `CHANGELOG.md` describes
the user-visible delta. GitLab and GitHub independently project equivalent
product trees into their own signed histories, tags, CI, assets, and Releases.
Neither Forge depends on the other.

The source Change closes after exact-HEAD proof and governed archival. Hosted
CI, independent Forge publication, asset comparison, installation, runtime
acceptance, and lane retirement remain separately verified delivery effects.
