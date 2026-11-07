## 1. Scrub

- [x] 1.1 Remove both peers' `forge_repository` from `.ethos/release.toml`;
      `ethos status` stays ready, and publish previews to both peers pass
      through the clone's Git configuration.
- [x] 1.2 Remove CHANGELOG's History rows and compare links, keeping its
      headings and text; `validate-changelog` and the Markdown link check pass.
- [x] 1.3 Cite the GitLab pipelines of the two archived task records by number,
      without a link.
- [x] 1.4 Give the tests that need a private forge fixture identities, and make
      the mirror-prelude test refuse any IPv4 literal and any
      `projects/<digits>` path; the changed tests pass on three platforms.

## 2. Verification

- [x] 2.1 Run the local quality graph at the lane head.
- [x] 2.2 Confirm the hosted three-platform graph on the proposal.
- [x] 2.3 Confirm that the tracked tree of the lane head contains none of the
      GitLab peer's identity forms.
