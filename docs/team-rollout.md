# Team rollout

## Maintainer

Commit a secret-free Profile catalog such as [`examples/team-profiles.toml`](../examples/team-profiles.toml):

```toml
version = 1
recommended_default = "dmx"

[profiles.dmx]
label = "DMXAPI"

[profiles.dmx.endpoints]
openai_responses = "https://www.dmxapi.cn/v1"
anthropic = "https://www.dmxapi.cn"
```

Validate it locally:

```bash
aigw config import team-profiles.toml
aigw config export
```

The manifest schema intentionally excludes tokens, personal routes, Adapter state and executable paths.

## Team member

Install a pinned Release, then:

```bash
aigw config import team-profiles.toml
aigw rotate dmx
aigw use dmx --all
aigw test
aigw doctor
```

Enable only the clients used on that workstation:

```bash
aigw adapter discover
aigw adapter enable claude --executable /absolute/path/to/claude
aigw adapter enable codex \
  --executable /absolute/path/to/codex \
  --target "$HOME/.codex/config.toml"
```

The AIGW installation directory must precede the raw Claude executable in `PATH`. AIGW reports the installed shim path but does not modify shell startup files.

## CI

Set `AIGW_SECRET_BACKEND=env` and inject a masked CI variable such as `AIGW_TOKEN_DMX`. The backend is read-only; `add`, `rotate`, rename and secret deletion fail rather than generating a local plaintext credential file.
