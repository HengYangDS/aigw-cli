## ADDED Requirements

### Requirement: Reviewed team configuration is directly consumable

The repository SHALL publish exactly one token-free team manifest containing
the reviewed Accounts, Profiles, and recommended client Routes. It SHALL be
directly consumable by `aigw setup --from` and SHALL NOT contain fictitious
providers, credentials, workstation paths, or a parallel example manifest.

#### Scenario: Team member imports reviewed settings

- **WHEN** a team member downloads the tracked manifest and runs `aigw setup --from`
- **THEN** AIGW SHALL import the reviewed DMXAPI, AIHubMix, and UCloud profiles
- **AND** SHALL request or reuse Tokens outside the manifest
- **AND** SHALL recommend GPT-5.6 Sol for Codex and Claude Fable 5 for Claude
