## 1. Regression-first fixes

- [x] 1.1 Add a GitHub workflow contract test requiring runner-temporary
  materialization of the allowed-signers variable in every provenance job.
- [x] 1.2 Add architecture policy tests proving POSIX roots, Windows drive,
  UNC/device roots, backslashes, and parent traversal are rejected identically
  on every host.
- [x] 1.3 Implement the minimal workflow and policy fixes and keep logs free of
  trust material.
- [x] 1.4 Restore the real Codex Home `config.toml` target and prove that one
  shared projection serves Codex CLI and Desktop without Desktop-only authority.
- [x] 1.5 Prove setup configures only discoverable Claude/Codex clients and
  leaves absent clients untouched; document Hermes as not yet admitted.
- [x] 1.6 Upgrade the Go module graph and GitHub Actions projections to current
  stable releases, preserve one owner for each version, and validate the
  resolved graph for known vulnerabilities.

## 2. Contract and proof

- [x] 2.1 Update the release policy and product specification to describe the
  effective hosted-CI trust-input and portable-path contracts.
- [ ] 2.2 Run focused tests, full local gates, strict OpenSpec validation, and
  exact-HEAD proof with pristine output.
- [ ] 2.3 Commit with the GitLab publication actor and trusted signature, land,
  replay the commit for GitHub, and publish with compare-and-swap controls.
- [ ] 2.4 Rerun exact-ref hosted Verify/Release pipelines before publication,
  deployment, runtime acceptance, or final housekeeping claims.
