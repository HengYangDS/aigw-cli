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
        reaching ETHOS `dev`. Owner: session "fix aigw and proxy"; next:
        inspect ETHOS `dev` for the accepted maintenance admission and the
        missing publication receipts of `v0.3.4` and `v0.3.5`; review
        2026-10-18.
  - [x] 1.2.4 Publish the maintenance tags `v0.3.4` and `v0.3.5`; read back
        GitHub's objects `e86777be` and `95132368`. Done by the user-authorized
        break glass of 2026-10-15, outside ETHOS publication.
- [x] 1.3 Supply GitLab's locked tool closure for the current
      `.config/mise/mise.lock` through upstream supply; GitLab's protected
      pipelines pass for the published head (`3cf22277`: `dev` 11342, `main`
      11343).
- [x] 1.4 Publish the GitLab Releases of `v0.3.4` and `v0.3.5` from their
      retained local bundles; `v0.4.2` stays without a GitLab Release by the
      user's decision of 2026-10-16.

## 2. Qualify GitLab Peer Admission

- [ ] 2.1 Qualify GitLab's effective status enforcement, unprotected proposals,
      guarded merges, signer trust, source-ref auto-delete and untrusted-runner
      isolation and cleanup, as `terminal-product-convergence` task 7.4 did for
      GitHub. Owner: session "fix aigw and proxy"; next: 2.1.2; review
      2026-10-18.
  - [x] 2.1.1 Make the native review probe count a protected resource behind a
        directory that the review identity cannot traverse as unreadable, while
        still requiring no elevation grant; the probe tests pass.
  - [ ] 2.1.2 Run the macOS review probe once with a pipeline-level
        `AIGW_REVIEW_PROTECTED_FILE`, then keep it as an unprotected project
        variable; the review job's trace carries the probe line.

## 3. Adopt The ETHOS Reference Successor

- [ ] 3.1 Once ETHOS accepts its identity and reference successor, qualify the
      installed admission: reachable wrong-repository links fail, and historical
      peer tag identities reconcile through authorized native repair without
      copied maps or checkers. Owner: session "fix aigw and proxy"; next: query
      ETHOS `dev` for the accepted identity and reference successor; review
      2026-10-18.
