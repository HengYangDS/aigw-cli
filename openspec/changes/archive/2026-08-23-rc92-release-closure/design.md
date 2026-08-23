## Decision

Create prerelease rc.92 from the current accepted source. Keep `VERSION` as the
single version authority and keep `CHANGELOG.md` as the forward release
chronicle. Describe the four accepted post-rc.91 corrections by user-visible
outcome rather than copying commit subjects.

After local source and release proof passes, archive this Change before
constructing the signed product commit. Construct one annotated tag locally and
publish the unchanged commit, tag, and immutable asset set to both peer Forges.
Installation and runtime acceptance remain separate evidence surfaces.
