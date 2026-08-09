# Design

Forge identity remains host configuration, never repository data. GitLab owns
the signer set as a protected file variable, so the job projects its generated
path directly into source verification. GitHub owns a value variable and uses
the repository trust-input command to materialize the equivalent job-local
file. The two Forge adapters stay independent while presenting one verifier
contract: a path to an allowed-signers file.

The Windows failure exposed an unnecessary host-specific policy in the portable
installer. A downloaded artifact is admitted by release verification before
installation; POSIX execute bits are neither portable nor a second integrity
check. The installer therefore accepts regular files on every platform,
continues to reject directories, and writes the target with product-owned mode.
