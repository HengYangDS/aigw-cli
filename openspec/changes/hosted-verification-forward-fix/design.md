# Design

Forge identity remains host configuration, never repository data. GitLab
projects the protected signer variable into a job-local file using the same
repository-owned trust-input command as GitHub, then passes only that file path
to source verification.

The Windows failure is a coverage gap, not a second installer design. The
platform-specific test follows the existing `Install` entry point and proves
that a source without meaningful POSIX mode bits is copied successfully.
