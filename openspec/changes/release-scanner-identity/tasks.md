## 1. Scanner Image

- [ ] 1.1 `executeTool` resolves, digests and runs the observed image of a call
      that requests it; a test runs the test binary and compares the digest,
      and an unresolvable name fails before anything starts.
- [ ] 1.2 The Mise lock reader returns the scanner's version, URL and SHA-256 for
      a platform from the repository's own lock, and refuses a missing tool,
      platform or checksum.
- [ ] 1.3 A scan refuses an image whose digest differs from the lock, a version
      report that differs from the locked version, and an image that changes
      between the version report and the scan, before writing either report.

## 2. Evidence

- [ ] 2.1 Both reports are schema 2 and name the scanner with portable
      arguments; the invocation receipt retains the exact arguments, the
      executable, the version report and the raw report's digest.
- [ ] 2.2 Provenance lists the locked scanner asset for schema 2 reports, adds
      nothing for schema 1, and refuses disagreeing reports, an inconsistent
      schema or an image that the lock does not select.
- [ ] 2.3 The network-declared dependency acceptance runs the real scanner on a
      fixture with the repository's lock and finds the host's locked image in
      both reports and the receipt.

## 3. Verification

- [ ] 3.1 `mise run check` passes on the lane head, and its own scan retains
      schema 2 reports and an invocation receipt naming the locked image.
- [ ] 3.2 The full proof of the lane head passes.
- [ ] 3.3 Hosted Verify passes for the proposal.
