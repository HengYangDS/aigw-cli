## 1. Permanent provenance mechanism

- [x] 1.1 Remove `.config/release/verified-commit-floors.txt` and every product,
  test, documentation, and governance reference to commit floors.
- [x] 1.2 Make commit provenance verify every commit reachable from `HEAD` for
  the selected external Forge actor and trust anchor.
- [x] 1.3 Add adversarial tests for an invalid root, invalid merge parent,
  wrong author or committer, untrusted or missing signature, mailmap overlay,
  missing context, and unknown provider.
- [x] 1.4 Make GitHub projection use complete-history verification and byte-exact
  message, tree, timestamp, parent-order, and merge-topology mapping.
- [x] 1.5 Test root and merge replay, multiline and unterminated message bytes,
  duplicate trees, non-canonical caller branches, unsupported source headers,
  isolated object storage, external signing context, and unchanged source refs.

## 2. Contract and local proof

- [x] 2.1 Update AGENTS, release policy, comments, and current docs so the
  described invariant matches the implemented whole-history mechanism.
- [x] 2.2 Modify the `product-control-plane` spec and bind the active Claim to
  this change without claiming remote publication or runtime acceptance.
- [x] 2.3 Validate OpenSpec strictly and run the focused provenance, GitHub
  projection, governance, portability, text, and architecture gates.
- [x] 2.4 Commit the mechanism with the external GitLab publication actor and a
  trusted signature; archive the completed change and rerun exact-HEAD proof.

The authorized history replacement, dual-Forge publication, installation,
runtime acceptance, and repository-family housekeeping are follow-up lifecycle
transactions. They consume this mechanism but do not become source-code tasks
inside the mechanism change.
