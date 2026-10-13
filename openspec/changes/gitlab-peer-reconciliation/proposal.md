# GitLab Peer Reconciliation

## Why

The intranet GitLab peer has been unreachable since 2026-10-13. Under the
operator's direction, AIGW converged and released through GitHub alone, as its
independent-peer policy permits. GitLab `dev` and `main` were last read at
`ef404988`, so GitLab lacks every later commit, the release tags `v0.3.4` and
`v0.3.5`, and each release GitHub receives before GitLab returns. Its protected
pipelines also fail before any product job: `mise install --locked` requests
the peer tool mirror package for the current lock's SHA-256, which was never
published and has no producer in AIGW.

`terminal-product-convergence` qualified peer admission on GitHub only (its
task 7.4). Its task 8.3.2 waits for an ETHOS identity and reference successor
that ETHOS has not yet accepted. Both obligations move here so that Change can
archive without claiming them.

## What Changes

- Publish the accepted branches and the attested release tags to GitLab
  through ETHOS, and read them back equal to GitHub.
- Supply GitLab's locked tool closure for the current lock through its
  declared peer mirror, or through upstream supply where its runners reach it.
- Qualify GitLab's effective status enforcement, guarded merges, signer trust,
  source-ref retirement and runner isolation, as task 7.4 did for GitHub.
- Adopt the accepted ETHOS identity and reference successor once it exists.

## Capabilities

`product-control-plane` gains the contract for a returning publication peer:
catch-up publishes the exact objects the other peer received, and the peer stays
incomplete until its own CI passes. The existing local-first publication
topology requirement is unchanged.

## Impact

Work runs only while the intranet GitLab peer is reachable, from an ETHOS work
lane in `~/projects/aigw-cli`. ETHOS owns branch and tag publication; AIGW adds
no publisher, mirror producer, copied tag map or reference checker. GitHub
history, published releases and the Homebrew cask are not rewritten. Who
produces the peer tool mirror is the operator's decision. Until this Change
completes, GitLab's pipeline results do not describe GitHub's releases.
