## 1. Reproduce and specify

- [x] 1.1 Confirm the hosted failure occurs in the installer-owned asset curl.
- [x] 1.2 Add a failing projection test for direct asset download, checksum
      verification, bounded HTTP/1.1 transport, and installer-script absence.

## 2. Implement

- [x] 2.1 Replace the installer bootstrap in the CUE authority.
- [x] 2.2 Regenerate the GitLab projection.

## 3. Verify

- [x] 3.1 Pass focused CI projection tests.
- [x] 3.2 Pass the complete local source and governance graph.
- [ ] 3.3 Prove the forward fix in hosted GitLab Linux jobs.
