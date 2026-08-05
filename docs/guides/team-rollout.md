# Team Rollout

A team distributes reviewed public configuration; each member supplies Tokens
locally.

```mermaid
flowchart LR
    R["Review manifest"] --> P["Publish token-free file"]
    P --> S["Member setup"]
    S --> K["Local OS credential store"]
    S --> C["Installed client projections"]
    C --> V["Member check"]
```

## Maintainer

1. Start from [`manifests/example.toml`](../../manifests/example.toml).
2. Add only reviewed Account endpoints and admitted Profiles.
3. Keep Tokens, personal paths, identities, and release credentials out.
4. Validate the manifest in a clean repository environment.
5. Publish it through the team's ordinary configuration channel.

A manifest should contain the minimum Profile set users need. Provider catalogs
are discovery input, not automatic routing policy.

## New member

```bash
aigw setup --from manifest.toml
aigw check
```

Setup:

- validates all public metadata first;
- prompts once per missing Account Token;
- writes Tokens only to the OS credential store;
- configures only installed admitted clients;
- rolls back AIGW-owned changes if a required projection fails.

## Existing member

Preview before merging reviewed metadata:

```bash
aigw config import manifest.toml --dry-run --json
aigw config import manifest.toml
```

| Collision | Default behavior | Explicit action |
| --- | --- | --- |
| Same semantic Account/Profile | Reuse | None |
| Same ID, different public metadata | Stop before mutation | Review and use the specific replace flag |
| Local-only Profile not in manifest | Preserve | Remove explicitly if obsolete |
| Existing Token | Preserve | Rotate explicitly if required |

Import does not change Routes unless the command explicitly requests that
operation.

## Client admission

The current release supports Codex and Claude Code. A rollout must not assume a
client is installed. Missing clients are reported and remain untouched.

Future clients require a separately reviewed adapter with:

- presence and version discovery;
- configuration or launch boundary;
- credential injection rule;
- rollback and uninstall proof;
- human and JSON diagnostics.

## Release installation

GitLab and GitHub are independent release sources. Verify one complete artifact
set from one source; do not mix a tag, checksum file, and archive across Forges.

| Platform | Preferred install |
| --- | --- |
| macOS | `.pkg` |
| Linux | `.deb` or `.rpm` |
| Windows | `.msi` |
| Offline/portable | Matching archive and checksum manifest |

After installation:

```bash
aigw --version
aigw doctor
aigw check
```

## Staged rollout

| Stage | Evidence |
| --- | --- |
| Manifest review | Token-free diff and semantic validation |
| Pilot | Clean install, setup, check, and rollback on each required platform |
| Team release | Protected Forge publication and artifact verification |
| Member adoption | Local setup/check results; no shared Token collection |
| Closeout | Deprecated manifest/profile references removed intentionally |

## CI boundary

CI uses synthetic endpoints, fixtures, and read-only environment Tokens. It
must not require a maintainer's OS credential store or real provider account.
