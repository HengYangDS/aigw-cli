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
  - [ ] 1.2.3 Record native ETHOS receipts for `v0.3.4` and `v0.3.5`, which
        both peers already name identically. AIGW is bound to runtime
        `2a682982` since 2026-10-20, whose preview still refuses with
        `proof_attestation_repository_policy_mismatch`: their sources predate
        the shared paradigm. Owner: ETHOS publication-history-repair 13.4.10
        (session "Ethos 框架分析"); next: bind the runtime that ships 13.4.10
        and preview both receipts again; review 2026-10-27.
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

- [x] 2.1 Qualify GitLab's effective status enforcement, unprotected proposals,
      guarded merges, signer trust, source-ref auto-delete and untrusted-runner
      isolation and cleanup, as `terminal-product-convergence` task 7.4 did for
      GitHub.
  - [x] 2.1.1 Make the native review probe count a protected resource behind a
        directory that the review identity cannot traverse as unreadable, while
        still requiring no elevation grant, and accept sudo's answer when the
        review identity lists its own empty grant; the probe tests pass.
  - [x] 2.1.2 Run the macOS review probe once with a pipeline-level
        `AIGW_REVIEW_PROTECTED_FILE`; the review job's trace carries the probe
        line (pipeline 11347, job 54275).
  - [x] 2.1.3 Keep `AIGW_REVIEW_PROTECTED_FILE` as an unprotected project
        variable after one macOS review job with it passes in full (pipeline
        11366, job 54355; kept 2026-10-18).
  - [x] 2.1.4 Record the signer-trust and Windows probe limits: protected
        pipelines verify pushed commits against the allowed-signers file, but
        GitLab cannot refuse an unsigned push without push rules, and Windows
        has no protected-file probe.
  - [x] 2.1.5 Retire the project variables `AIGW_GITLAB_LINUX_RUNNER_TAG`,
        `AIGW_GITLAB_DARWIN_RUNNER_TAG` and `AIGW_GITLAB_WINDOWS_RUNNER_TAG`,
        which no job reads since `v0.3.3`, after both Linux runner paths ran the
        current workflow (pipelines 11357 and 11366; retired 2026-10-18).

## 3. Adopt The ETHOS Reference Successor

- [ ] 3.1 Once ETHOS accepts its identity and reference successor, qualify the
      installed admission: reachable wrong-repository links fail, and historical
      peer tag identities reconcile through authorized native repair without
      copied maps or checkers. ETHOS publication-history-repair task 13.4.9
      owns reconciling the eleven rc tags; their per-peer objects are in
      `build/evidence/gitlab-peer-reconciliation/rc-tag-identities-20261020/`
      of the main checkout. Owner: session "fix aigw and proxy"; next: qualify
      once 13.4.9 ships in an aigw runtime; review 2026-10-27.
