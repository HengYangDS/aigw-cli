# AIGW Client Projection Edition Provider

This directory owns one explanatory Client Projection chapter, not the whole
AIGW product. AIGW owns its facts, editorial choices, acceptance and release.
Architecture Publisher compiles the selected inputs without granting approval.

## Inputs and ownership

| Path                                      | Responsibility                                                                               |
| ----------------------------------------- | -------------------------------------------------------------------------------------------- |
| [claim-model.json](claim-model.json)      | Authored entities, typed relations and claims, with exact native-source provenance.          |
| [edition.json](edition.json)              | Four reader questions, scope, narrative and independent static/interactive views.            |
| [provider.json](provider.json)            | Declarative `architecture.edition-provider/v2` selection of those two files.                 |
| [build-request.json](build-request.json)  | Public `architecture.build-request/v1` selecting one Provider and both media.                |
| [selection.json](selection.json)          | Exact published compiler package and Git-bound predecessor for rollback.                     |
| [source tests](test/source.test.mjs)      | Selected byte identity, live provenance, preserved meaning and rollback availability.        |
| [installed test](test/installed.test.mjs) | Exact package, relocated offline replay, direct/declarative input parity and tamper refusal. |

The Claim Model's `source/` locators identify repository-relative provenance;
they are not retained copies. Source tests resolve those locators to live tracked
owners. Publisher reports internal Claim Model provenance as unverified: byte
consistency does not certify implementation truth. The model remains native
AIGW-authored input, not a Publisher-derived projection.

## Reproduce

The repository's locked quality graph runs the source tests. To qualify the
published compiler independently, supply its exact archive and release manifest:

```bash
ARCHITECTURE_PUBLISHER_ARCHIVE=/absolute/path/to/selected-package.tgz \
ARCHITECTURE_PUBLISHER_RELEASE_MANIFEST=/absolute/path/to/release-manifest.json \
TMPDIR=/absolute/path/to/owned/scratch \
  mise exec --locked -- node --test architecture/edition-provider/test/installed.test.mjs
```

Repository-wide `ethos prove --execute` also selects the installed test. Set
both archive variables for that invocation and verify their hashes against
[selection.json](selection.json) before starting the proof. Missing inputs
reject the run; the source-only quality graph does not qualify this package.

This installs only the selected local archive in an isolated consumer, offline
and without lifecycle scripts. No neighboring Publisher checkout is imported.
The installed test exercises the public `archpub edition build` entry point;
there is no AIGW Provider command or generic materialization controller.

For a normal build, resolve the exact installed compiler selected by
`selection.json`, compute the Build Request's SHA-256, and invoke:

```bash
archpub edition build /absolute/path/to/build-request.json \
  --sha256 <exact-request-sha256> --output /absolute/path/to/new-candidate
```

Output is an unqualified Candidate, not accepted source or a release. Direct and
declarative modes preserve identical selected contents and media, but their
closure metadata and Candidate identities are not interchangeable approval.
Generated locks, Candidates and media belong in owned untracked output, not
beside source inputs as another authority.

## Change and rollback

When selected source meaning changes, update its Claim Model provenance and
editorial selections deliberately. Update the Provider's exact file sizes and
hashes, then the Build Request's Provider hash. The compiler owns derived locks
and Candidate identities; AIGW does not maintain copies of them.

`selection.json` pins the signed predecessor commit and complete Provider tree.
Git can reconstruct that entire tree, including its original compiler selection
and captured provenance, in an owned rollback checkout. The pre-migration
installed replay is retained as evidence; do not reinterpret v1 bytes as v2 or
restore obsolete copies into the live owner. Native source and installed tests
must pass before retiring replaced inputs.
