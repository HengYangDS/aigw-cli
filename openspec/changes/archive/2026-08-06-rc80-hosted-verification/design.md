## Context

GitLab and GitHub intentionally carry independently signed commit histories.
GitHub rc.79 nevertheless tried to resolve a GitLab commit ID from tracked
evidence, even though the same product tree existed under a GitHub-native
commit. The Windows matrix also used `go.exe` as a launcher fixture. GitLab
additionally needs the protected trust anchor for the authorized publication
key; that is external release context, not repository source.

## Goals / Non-Goals

**Goals:**

- Make evidence identity provider-neutral while preserving strict local history
  proof in each Forge.
- Make the Windows launcher test hermetic and product-semantic.
- Preserve strict provenance while advancing only to rc.80.

**Non-Goals:**

- Adding compatibility behavior, provider branches, Proxy ownership, or local identity.
- Rewriting rc.79 tags, runs, Releases, or historical evidence.

## Decisions

1. Treat the recorded tree as the portable content identity. Verification must
   find that tree in the current `HEAD` ancestry. If the recorded commit object
   is locally available, its tree must still match; if it belongs only to the
   peer Forge, its absence is not an error. Cross-Forge fetching and commit-ID
   mapping are rejected because they couple independent publication planes.
2. Hard-link the current Go test executable into a test-owned persistent path;
   it has one controlled identity and no dependency on an unrelated host tool.
   Borrowing `go.exe` is rejected because the launcher validator correctly
   treats it as foreign.
3. Supply GitLab signer trust through protected CI variables/files. No author,
   email, fingerprint, key, or host path enters product source.

## Risks / Trade-offs

- **Tree search walks history** → use the already complete local Forge history
  and stop at the first matching tree.
- **Temporary Unix paths are deliberately rejected by the product** → place
  the disposable test-owned link under the user home and remove it in cleanup.

## Migration Plan

Run focused tests and workflow contracts, then the complete local gate and
exact-HEAD proof. Archive, land, create signed Forge-specific equal-tree commits,
and publish rc.80 only after both independent pipelines pass.
