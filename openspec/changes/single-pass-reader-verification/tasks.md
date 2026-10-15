# Tasks

## 1. Verify Once

- [x] 1.1 Inject a `credential.Files` capability through the App, invocation
      and synchronizer, and add a failing regression that counts executable
      opens per path in one `setup`, `sync` and `use --for` command; it fails at
      the current two program and two reader reads per command.
- [x] 1.2 Read the program once through `credential.ReadProgram`, verify or
      install each reader once through `Program.EnsureEntrypoint`, and confirm
      it after projection through `Entrypoint.Confirm` without another read; the
      regression passes, and `TestExecutableDigestRechecksConcurrentSources` and
      every existing reader integrity test still pass.
- [x] 1.3 Keep no program bytes in memory: stream verification, and on
      installation copy the program again only when that copy hashes to the
      verified digest; the counting regression expects one program read, or
      two when it installs, and memory-bounded verification still passes.
- [x] 1.4 Make `Entrypoint.Confirm` stream the reader again and refuse bytes
      whose digest differs from the verified one; a same-size in-place rewrite
      with a restored timestamp is refused, and the counting regression expects
      two reader reads.
- [x] 1.5 Read executables through 64 KiB pooled buffers, so one command stays
      below 256 KiB even when a collection empties the pool; the memory test
      empties the pool before every operation (GitHub dev push run 38758007867
      failed macOS at 272,953 B/op with 256 KiB buffers).

## 2. Requalify

- [ ] 2.1 Measure all eight native workloads against the published 0.4.1
      predecessor on macOS, Linux and Windows; every candidate p95 stays within
      its budget or the exceedance is explicitly accepted. The 0.4.2 release
      artifacts are the measured candidate.
