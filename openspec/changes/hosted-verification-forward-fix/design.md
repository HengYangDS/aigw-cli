# Design

Forge identity remains host configuration, never repository data. GitLab owns
the signer set as a protected file variable, so the job projects its generated
path directly into source verification. GitHub owns a value variable and uses
the repository trust-input command to materialize the equivalent job-local
file. The two Forge adapters stay independent while presenting one verifier
contract: a path to an allowed-signers file.

The Windows failure is a coverage gap, not a second installer design. The
platform-specific test follows the existing `Install` entry point and proves
that a source without meaningful POSIX mode bits is copied successfully.
