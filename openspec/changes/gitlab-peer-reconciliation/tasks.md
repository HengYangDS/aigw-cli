# Tasks

## 1. Catch GitLab Up

- [x] 1.1 Publish accepted `dev` and `main` to GitLab through ETHOS; read both
      refs back equal to GitHub's.
- [ ] 1.2 Publish every attested release tag that GitHub received during the
      outage, `v0.3.4` and later, to GitLab through ETHOS; read each tag object
      back equal to GitHub's.
  - [x] 1.2.1 Publish `v0.4.0` and `v0.4.1`, whose release commits are in
        accepted history; read back GitHub's objects `6be2e3d3` and `c9f923b9`.
  - [x] 1.2.2 Remove the root `.cbmignore`, which ETHOS's repository paradigm
        rejects.
  - [ ] 1.2.3 Rebind aigw-cli to an accepted ETHOS runtime that admits attested
        maintenance releases; depends on `work/20261002-native-trust-closure`
        reaching ETHOS `dev`.
  - [ ] 1.2.4 Publish the maintenance tags `v0.3.4` and `v0.3.5`; read back
        GitHub's objects `e86777be` and `95132368`.
- [ ] 1.3 Supply GitLab's locked tool closure for the current
      `.config/mise/mise.lock`, through the peer mirror or the operator's chosen
      upstream supply; GitLab's protected pipeline passes for the published head.
      Upstream supply passes every job except `quality`, which waits for the
      `v0.3.5` tag of task 1.2.4.

## 2. Qualify GitLab Peer Admission

- [ ] 2.1 Qualify GitLab's effective status enforcement, unprotected proposals,
      guarded merges, signer trust, source-ref auto-delete and untrusted-runner
      isolation and cleanup, as `terminal-product-convergence` task 7.4 did for
      GitHub.

## 3. Adopt The ETHOS Reference Successor

- [ ] 3.1 Once ETHOS accepts its identity and reference successor, qualify the
      installed admission: reachable wrong-repository links fail, and historical
      peer tag identities reconcile through authorized native repair without
      copied maps or checkers.
