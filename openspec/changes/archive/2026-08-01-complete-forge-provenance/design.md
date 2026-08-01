## Context

The current verifier starts after `.config/release/verified-commit-floors.txt`.
That makes a later clean suffix appear trusted even when an earlier reachable
commit has the wrong actor or no trusted signature. The current GitHub
projection is append-only and also delegates its trust boundary to those
floors. The repository therefore has a suffix policy, while the product claim
requires complete reachable provenance.

This change separates two concerns:

1. the permanent product mechanism that verifies and projects complete
   Forge-specific history; and
2. the authorized publication operation that replaces already exposed invalid
   history across both Forges.

## Goals / Non-Goals

**Goals:**

- Make every reachable commit part of the provenance invariant.
- Preserve source semantics exactly while changing only Forge-owned identity
  and signature data.
- Construct and verify replacement graphs away from canonical object storage.
- Make replacement recoverable, concurrency-safe, and complete across every
  affected publication surface.
- Keep execution-specific identity and key material outside reusable source.

**Non-Goals:**

- Sharing Git object IDs between identity domains.
- Treating `.mailmap`, a new root commit, or an allowlist floor as repair.
- Providing a general-purpose history editor or an automatic force-push path.
- Mutating application-managed Codex state.

## Decisions

### Reachability is the permanent trust boundary

`check-commit-provenance.sh` verifies every commit in `rev-list --reverse
--topo-order HEAD`. The selected Forge's external publication context supplies
its required actor email and allowed-signers file. The check rejects a mailmap,
an invalid actor, an untrusted signature, an unknown provider, or an empty
trust input. There is no repository floor and no optional range argument.

Alternative: retain a bootstrap floor. Rejected because it converts a known
invalid prefix into trusted reachable history and makes the verdict weaker than
its name.

### Semantic replay is one minimal primitive

A replay maps each source commit to one target commit in topological order. For
each object it preserves:

- the exact tree object;
- the exact commit-message byte sequence;
- author and committer timestamps;
- ordered parent relationships and merge arity.

It replaces author and committer identity with the selected Forge actor and
adds that Forge's trusted signature. Parent object IDs necessarily change and
are resolved only through the replay map. A verifier compares the two graphs by
ordered topology and semantic fields, not by commit ID.

Alternative: `filter-branch`, message extraction through shell variables, or
per-branch cherry-pick. Rejected because those forms can lose message bytes,
flatten topology, duplicate work, or leave refs inconsistent.

### Object isolation precedes authority mutation

Replay runs in a fresh object database with no alternates, linked worktree, or
shared Git common directory. It emits an old-to-new map and a complete
verification receipt before a canonical ref can move. Reusable source does not
contain an operator path: the caller supplies temporary and record locations.

Alternative: rewrite the canonical repository in place and roll back refs on
failure. Rejected because newly written objects, partial ref movement, hooks,
and concurrent readers make the failure boundary unnecessarily broad.

### Each Forge owns one complete graph

GitLab and GitHub retain independent actor identities and signatures. GitLab's
rebuilt graph is the canonical source history. GitHub is replayed from the
canonical semantic graph and must match its ordered trees, messages, timestamps,
and topology. Steady-state GitHub projection uses the same mapping semantics
and verifies the complete existing GitHub graph before it appends anything.

Alternative: copy GitLab objects to GitHub. Rejected because that destroys the
explicit identity-domain contract.

### Publication replacement is fail closed

Before replacement, repository-family recovery admission records exact local
refs, affected remote refs, tags, release identities, expected old tips, and
the replay inputs. Replacement uses compare-and-swap locally and
force-with-lease remotely against those captured tips. Any changed peer tip,
missing mapping, failed signature, hosted-CI failure, incomplete release, asset
mismatch, or stale evidence binding blocks completion.

The transaction is complete only when affected branches and annotated tags map
to verified objects on both Forges; hosted CI is green at the exact tips;
release records and assets are rebuilt and byte-compared; active commit-bound
evidence is refreshed; and the recovery record has a verified manifest.

Alternative: replace `main`, move the floor, and repair tags later. Rejected
because it exposes a mixed provenance state and makes rollback ambiguous.

### Publication context is external data

Actor names and emails, allowed signers, signing keys or agent fingerprints,
signing programs, remote names or URLs, and repository-family record paths are
required inputs. Tests use generated fixture identities and temporary paths.
No operator-specific value becomes a source default.

## Risks / Trade-offs

- **Every commit receives a new OID** -> retain the complete mapping and old ref
  inventory in an immutable recovery record; update active SHA-bound evidence.
- **Remote history replacement disrupts existing clones** -> publish one clear
  migration notice and require a fresh clone or explicit verified reset.
- **Signing hundreds of commits is slow** -> replay once per Forge in isolated
  storage, then verify the frozen result; never re-sign during ref publication.
- **A remote changes during preparation** -> force-with-lease fails and the
  publication transaction remains incomplete without overwriting the peer.
- **Historical releases point into replaced history** -> rebuild every affected
  annotated tag and release record, never copy provider-native tag objects.

## Migration Plan

1. Land the permanent whole-history checker, replay/projection semantics, tests,
   documentation, and OpenSpec contract as a signed mechanism change.
2. Admit an immutable recovery record and capture exact local and remote
   branches, tags, releases, inputs, and expected old tips.
3. Replay GitLab history in an isolated object database; emit its map and prove
   whole-history actor, signature, tree, message, timestamp, and topology
   invariants.
4. Refresh active commit-bound claims against the rebuilt GitLab graph, create
   their signed commit, and rerun exact-HEAD local proof.
5. Replay the final canonical graph into the GitHub identity domain and prove
   the same semantic invariants plus its independent actor and signatures.
6. Replace affected local and remote refs with compare-and-swap and
   force-with-lease, rebuild provider-native tags and releases, and run hosted
   CI at both exact tips.
7. Download and compare the complete release matrices, verify the recovery
   record, publish clone-migration guidance, then perform owner-bound lane and
   worktree closeout.

Rollback before remote replacement discards the isolated object stores.
Rollback after any remote replacement restores only the captured old refs with
compare-and-swap and then restores the corresponding release records; it never
silently mixes old and new graphs.
