## 1. Scrub

- [x] 1.1 Remove the GitLab peer's `forge_repository` from `.ethos/release.toml`;
      `ethos status` stays ready, and a GitLab publish preview passes through
      the clone's `remote.gitlab.forgeRepository`.
- [x] 1.2 Remove CHANGELOG's GitLab compare links; `validate-changelog` and the
      Markdown link check pass.
- [x] 1.3 Cite the GitLab pipelines of the two archived task records by number,
      without a link.
- [x] 1.4 Give the tests that need a private forge fixture identities, and make
      the mirror-prelude test refuse any IPv4 literal and any
      `projects/<digits>` path; the changed tests pass on three platforms.

## 2. Verification

- [ ] 2.1 Run the local quality graph at the lane head.
- [ ] 2.2 Confirm the hosted three-platform graph on the proposal.
- [ ] 2.3 Confirm that the tracked tree of the lane head contains none of the
      GitLab peer's identity forms.
