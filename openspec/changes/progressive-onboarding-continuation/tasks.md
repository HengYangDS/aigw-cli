## 1. Deferred activation

- [x] 1.1 Add an acceptance regression for one connected Account with no
      installed clients and verify it fails on the current `aigw status`
      continuation.
- [x] 1.2 Make the existing setup result select `aigw sync` and verify the
      focused setup acceptance suite passes.
- [x] 1.3 Add an acceptance regression for setup without clients followed by
      client installation and `aigw use`, and verify the current adapter remains
      disabled.
- [x] 1.4 Reuse one synchronization-domain client convergence function from
      `sync`, `repair`, and `use`; verify `use` activates only its Profile's
      declared client and rolls back on projection failure.

## 2. Closure

- [x] 2.1 Validate the OpenSpec Change and run the affected source gate.
- [ ] 2.2 Run exact-HEAD proof, archive the Change, land it, and verify accepted
      source, hosted CI, publication, installed runtime, and lane retirement.
