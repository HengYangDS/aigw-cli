# Spec Delta

## MODIFIED Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be independent Adapters owning discovery,
projection, authentication, rollback, verification, status, and withdrawal of
AIGW state. Account-Token credential commands MUST use an absolute qualified
reader and Route without storing Tokens in client files. A package-manager
CLI link MUST NOT be assumed continuously available during replacement. Client-native
authentication and preferences SHALL remain client-owned. Verification SHALL
use synchronized settings without wrappers. New clients MUST add an Adapter
without changing provider policy or existing Adapters.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: Claude launches outside the installer shell

- **WHEN** Claude Code requests a credential from an enabled AIGW projection
- **THEN** `apiKeyHelper` SHALL invoke the selected qualified reader
- **AND** credential retrieval SHALL not depend on the caller's PATH
- **AND** the projected settings SHALL contain no plaintext Token

#### Scenario: Codex authenticates an explicit native provider

- **WHEN** an enabled Codex Route selects an explicit native provider identity
- **THEN** its declared authentication mode SHALL determine credential ownership
- **AND** Account-Token authentication SHALL invoke the selected qualified
  reader for only the active Codex Route
- **AND** client-native authentication SHALL project no AIGW Token helper
- **AND** neither mode SHALL write a plaintext Token to public configuration

#### Scenario: Claude uses an Anthropic-compatible provider

- **WHEN** explicit verification invokes Claude Code for an admitted Route
- **THEN** the native client SHALL consume its synchronized settings in bare,
  nonpersistent mode
- **AND** stale AIGW-owned Anthropic environment overrides SHALL be removed
- **AND** unrelated client preferences, including beta controls, SHALL remain
  unchanged rather than becoming a new AIGW authority

#### Scenario: The installed executable path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the AIGW executable path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: The credential reader path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the selected reader path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: Credential retrieval is not admitted

- **WHEN** a credential request names an unsupported client, a disabled adapter,
  an unresolved Route, or an Account without a Token
- **THEN** the request SHALL fail without writing credential bytes to standard
  output

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, external-gateway behavior,
  command roots, or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW SHALL project the selected Route once into that shared home
- **AND** SHALL NOT create a second Desktop-specific configuration authority

#### Scenario: An adapter is proposed for another agent client

- **WHEN** a new agent client such as pi or OpenCode is proposed
- **THEN** its admission SHALL independently prove executable discovery,
  protocol/model selection, credential ownership, native configuration merge,
  rollback, withdrawal and a real tool-loop journey
- **AND** AIGW SHALL NOT advertise support based only on a writable config
  path or a successful endpoint probe.

### Requirement: Reviewed team configuration is directly consumable

The repository SHALL publish one token-free reviewed manifest directly
consumable by `aigw setup --from` without credentials or installed clients.
Setup and sync SHALL preserve existing Client Bindings and fill only unselected
bindings from declared primary Routes and ordered alternatives with usable
authentication. A Route outside that recommendation SHALL require explicit
selection, even if its Account is connected or its Model matches. They SHALL
project only AIGW-owned state and never expose or rebind Tokens. Fictitious
providers, workstation paths, and parallel example manifests SHALL NOT remain.

#### Scenario: Team member imports reviewed settings

- **WHEN** a team member downloads the tracked manifest and runs `aigw setup --from`
- **THEN** AIGW SHALL import the Accounts and Routes from the reviewed
  `manifests/team.toml` without a second provider or model-name policy
- **AND** required Account Tokens SHALL remain outside the manifest
- **AND** recommended selections SHALL come from that manifest rather than
  duplicated model-version literals in this specification.

#### Scenario: No Account is connected during import

- **WHEN** a user imports the team manifest without supplying a Token
- **THEN** every reviewed Account and Route SHALL be retained
- **AND** no client installation or credential SHALL be required
- **AND** the next action SHALL enumerate the compatible Account connection
  choices without making one Account mandatory.

#### Scenario: One Provider Account is connected

- **WHEN** a user imports the team manifest with exactly one available Account Token
- **THEN** setup SHALL succeed without Tokens for other Accounts
- **AND** each unselected client SHALL select the first usable declared
  recommendation compatible with that client and connected Account
- **AND** an undeclared Route SHALL remain unselected rather than becoming a
  Model or lexical fallback.

#### Scenario: A compatible Account becomes available after import

- **WHEN** setup retained the reviewed catalogue without a connected Account
- **AND** a Token for a declared compatible recommendation later becomes
  available through the read-only environment backend
- **THEN** `aigw sync` SHALL select that recommendation for an unselected client
- **AND** SHALL preserve the declared protocol and existing Client Bindings
- **AND** SHALL NOT require Tokens for other Accounts.

#### Scenario: A supported client is installed later

- **WHEN** setup completed before an admitted client was installed
- **AND** its selected Route has usable declared authentication
- **THEN** `aigw sync` SHALL discover and project that client
- **AND** SHALL NOT require, replace, or expose any unrelated Token
- **AND** SHALL leave absent clients untouched.

### Requirement: Online update owns its temporary resources and preserves failure causes

An online update SHALL own all peer download directories within one operation
workspace and attempt to remove that workspace on every return. Cleanup SHALL
preserve unrelated files and SHALL NOT alter the completed installation outcome.
Reported failures SHALL retain their original causes and exact owned paths for
internal caller inspection, while public diagnostics SHALL expose only safe
state and recovery guidance. Forge subprocesses SHALL disable native prompts
explicitly rather than depend on the parent environment.

The updater SHALL authenticate each complete peer's checksum manifest against
the independently embedded release public key before checksum validation,
extraction, or candidate execution. Downloaded metadata SHALL NOT supply trust.
Release construction SHALL freeze that public key once and use the same value
for executable embedding and verification of the actual checksum signature.

#### Scenario: A checksum-consistent candidate lacks valid authentication

- **WHEN** an online peer supplies an archive and matching checksum manifest
- **AND** its signature is absent, malformed, signed by another key, or bound
  to different manifest bytes
- **THEN** the updater SHALL reject it before any candidate execution
- **AND** the current program and retained rollback bytes SHALL stay unchanged
- **AND** its exact owned download workspace SHALL be removed.

#### Scenario: Signing inputs change after release admission

- **WHEN** release construction froze the public key embedded in its programs
- **AND** another key signs the actual checksum manifest
- **THEN** construction SHALL reject the output before replacing accepted assets
- **AND** the prior accepted output SHALL remain unchanged.

#### Scenario: A release peer is temporarily unavailable

- **WHEN** a configured peer fails during metadata or asset transport
- **THEN** the updater SHALL apply the same unavailability classification for
  GitLab and GitHub and continue with another admitted peer
- **AND** if every peer is unavailable, the error SHALL retain every peer's cause
- **AND** authentication or integrity failures SHALL remain terminal.

#### Scenario: Workspace cleanup fails

- **WHEN** the operation's workspace cannot be fully removed
- **THEN** the error SHALL preserve its exact workspace and cleanup cause for
  internal inspection together with any preceding failure
- **AND** public output SHALL omit private paths and raw subprocess diagnostics
- **AND** if replacement completed, the diagnostic SHALL state that the program
  was updated rather than imply an unchanged or rolled-back installation.

#### Scenario: Replacement staging cleanup fails after activation

- **WHEN** a candidate or rollback program was activated but its staging
  directory cannot be removed
- **THEN** the program transition SHALL retain both its completed activation
  result and the cleanup failure
- **AND** public output SHALL report the active replacement and incomplete
  cleanup without claiming that program replacement failed.

## ADDED Requirements

### Requirement: Diagnostic quota and throttling remain distinct

An authenticated diagnostic SHALL use explicit provider error evidence, not HTTP
status alone, to distinguish exhausted Account credit or quota from transient
rate or concurrency limits. Classification SHALL NOT switch Routes or retry a
request automatically.

#### Scenario: A provider reports hard quota exhaustion with HTTP 429

- **WHEN** an Account diagnostic receives HTTP 429 with an unambiguous exhausted
  quota code or insufficient balance evidence
- **THEN** AIGW SHALL report non-retryable Account quota exhaustion and give
  Account-scoped recovery guidance

#### Scenario: A provider reports transient throttling with HTTP 429

- **WHEN** an Account diagnostic receives HTTP 429 with rate-limit or ambiguous
  concurrency-quota evidence
- **THEN** AIGW SHALL report retryable throttling rather than exhausted credit
- **AND** the diagnostic SHALL have made only its single admitted request.

### Requirement: Client projection failure reports verified recovery state

After a configuration commit, a failed native client projection or credential
entrypoint finalization SHALL report whether AIGW configuration restoration
succeeded. Human and JSON output SHALL
use the same outcome and safe next action without exposing a private file path,
credential value, or internal error. Configuration restoration SHALL NOT be
presented as proof that every client or credential entrypoint was restored.

#### Scenario: Configuration was restored after projection failure

- **WHEN** a client file rejects the projection and the configuration snapshot
  is restored
- **THEN** the error SHALL say that configuration was restored
- **AND** SHALL direct the operator to inspect client state with `aigw doctor`
  before retrying.

#### Scenario: Configuration restoration is incomplete

- **WHEN** a client projection fails and restoring the configuration fails
- **THEN** the error SHALL report incomplete restoration rather than claim a
  successful rollback
- **AND** SHALL not expose the underlying file path or imply retry is safe.

#### Scenario: Credential entrypoint finalization fails after projection

- **WHEN** client projection succeeds but its credential entrypoint fails
  finalization and compensation is attempted
- **THEN** the error SHALL report the observed configuration restoration result
- **AND** failed client or entrypoint compensation SHALL not conceal that result
- **AND** public output SHALL retain safe recovery guidance without private paths.
