## 1. Curation

- [ ] 1.1 Record the owner's curation of 2026-11-12 in the
      progressive-team-onboarding delta: Anthropic, OpenAI and Google by the
      owner's list, and every other vendor by its most capable generally
      available model; strict validation passes.

## 2. Qualification

- [ ] 2.1 Qualify Claude Fable 5.2 on each provider and protocol that serves it
      with a native Hermes tool turn and a resumed recall, and add every
      model that passes to `manifests/team.toml`.
- [ ] 2.2 Qualify Gemini 4 Carbon, and the DMXAPI SSVIP variants of Gemini 4
      Argon and Carbon, the same way, and add every model that passes.
- [ ] 2.3 Qualify plain-DMXAPI `gpt-6.1-terra` and `claude-haiku-5-5` the same
      way, and add each one that passes.
- [ ] 2.4 For every other vendor, read its release notes and the providers'
      catalogs, name its most capable generally available model, qualify a
      successor and replace the family; Doubao has one family; the research
      record cites each choice.

## 3. Verification

- [ ] 3.1 `aigw setup --from manifests/team.toml` applies the refreshed file in
      its test, and the full check passes.
