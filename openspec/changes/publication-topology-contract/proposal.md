# Declare the publication topology

## Why

The release policy already names independent GitLab and GitHub remotes, but it
does not declare the repository-native local verification, local installation,
or CI surfaces consumed by publication admission.

## What changes

- bind local verification to the existing full local CI command;
- bind local installation to the existing portable bundle installer;
- name each Forge's existing CI surface independently;
- make the existing governance check reject an incomplete declaration.

## Out of scope

- a new release command, installer, compatibility path, or CI workflow;
- remote mutation, tag creation, or release publication.
