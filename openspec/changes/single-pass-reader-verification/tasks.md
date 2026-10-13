# Tasks

## 1. Verify Once

- [ ] 1.1 Add a failing regression that counts executable reads in one
      `setup`, `sync` and `use --for` command; it fails at the current three to
      four reads per command.
- [ ] 1.2 Carry the program and reader digests through one command so each file
      is read once; `TestExecutableDigestRechecksConcurrentSources` and every
      existing reader integrity test still pass.

## 2. Requalify

- [ ] 2.1 Measure all eight native workloads against the published 0.4.0
      predecessor on macOS, Linux and Windows; every candidate p95 stays within
      its budget or the exceedance is explicitly accepted.
