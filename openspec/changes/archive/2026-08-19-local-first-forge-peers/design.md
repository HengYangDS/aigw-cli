## Context

See [proposal.md](proposal.md). The current implementation clones and rewrites
the complete local graph with `commit-tree`, stores replay maps, creates
provider-qualified tag namespaces, and accepts tree equality when commit
identity differs. The product already has one clean local Git database and can
verify SSH-signed objects without consulting a Forge.

## Goals / Non-Goals

**Goals:**

- one semantic owner for commit and tag construction;
- exact object identity across local Git, GitLab, and GitHub;
- independent peer failure, retry, credentials, CI, and Release records;
- a complete zero-Forge local lifecycle;
- a small provider-neutral developer surface.

**Non-Goals:**

- rewriting historical formal releases merely to normalize old mistakes;
- managing branch protection, Forge credentials, or hosted account keys inside
  product source;
- duplicating ETHOS branch lifecycle or changing AIGW runtime behavior.

## Decisions

### Local Git owns product objects

Commits and annotated tags are constructed and signed once in the canonical
local repository. A peer projector may verify, observe, push, and re-observe
those objects; it may not clone another peer, rewrite identity, construct a new
object, or sign again.

Alternatives rejected: Forge-specific replay changes object identity and
creates two histories; detached source bundles still create a second object
owner without solving publication identity.

### Remote name selects transport, not product semantics

The projector accepts a configured Git remote plus product email and an
explicit allowed-signers file. GitLab/GitHub labels, actor names, signing keys,
and target signer inputs are absent because transport credentials belong to Git
and the host credential manager.

### Publication is exact compare-and-swap

Publishing `main` atomically projects the same local commit to remote `main`
and `dev`; `proposal/*` projects only its matching ref. New refs use a zero-OID
lease. Fast-forward and idempotent updates need no destructive authorization.
A divergent ref requires its exact currently observed OID and uses
`--force-with-lease`. Every target ref is re-read after push.

### Product trust and hosted verification are distinct

SSH object verification uses the product trust anchor. A Forge's `Verified`
badge is an account projection and cannot authorize object reconstruction.
Additional organizational approval belongs in detached attestations.

### Remove parallel lifecycle ownership

`sync`, `closeout`, and `promote-release` are deleted. Local branch transitions
belong to ETHOS governance; this repository retains only product-specific
object verification and optional peer publication.

## Risks / Trade-offs

- **Existing peer histories diverge** -> perform one bounded cutover using
  fresh exact remote tips, then restore protected-branch force-push policy.
- **Old formal tags have different objects** -> preserve released history until
  an explicit inventory distinguishes formal releases from failed intermediate
  artifacts; all new tags use exact identity.
- **One peer is unavailable** -> report that peer incomplete while local and the
  other peer remain independently operable.

## Migration Plan

1. Replace canonical requirements and tests.
2. Delete replay and lifecycle code; implement exact verification and
   publication.
3. Regenerate CI from its existing CUE authority and pass local gates.
4. Archive the Change and create one signed local product commit.
5. Cut over each peer independently with fresh leases and immediately restore
   protection.
6. Verify exact refs, hosted CI, release objects, assets, installation, and
   runtime behavior before retiring obsolete refs and worktrees.
