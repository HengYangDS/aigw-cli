# Team rollout

## Maintainer

维护一个不含 Token 的团队 Account + Profile 清单，例如 [`examples/team-profiles.toml`](../examples/team-profiles.toml)：

```toml
version = 1
recommended_default = "gpt-5.6-sol-cdx"

[accounts.dmx]
label = "DMXAPI"

[accounts.dmx.endpoints]
openai_responses = "https://www.dmxapi.cn/v1"
anthropic = "https://www.dmxapi.cn"

[accounts.dmx.account_probe]
kind = "dmxapi"
base_url = "https://www.dmxapi.cn"

[profiles."gpt-5.6-sol-cdx"]
label = "GPT-5.6"
account = "dmx"
client = "codex"

[profiles."gpt-5.6-sol-cdx".models]
codex = "gpt-5.6-sol-cdx"

[profiles."gpt-5.5"]
label = "GPT-5.5"
account = "dmx"
client = "codex"

[profiles."gpt-5.5".models]
codex = "gpt-5.5"

[profiles."gpt-5.5-ssvip"]
label = "GPT-5.5 SSVIP"
account = "dmx"
client = "codex"

[profiles."gpt-5.5-ssvip".models]
codex = "gpt-5.5-ssvip"

[profiles."claude-sonnet-5"]
label = "Claude Sonnet"
account = "dmx"
client = "claude"

[profiles."claude-sonnet-5".models]
claude = "claude-sonnet-5"

[profiles."claude-opus-4-8-thinking"]
label = "Claude Opus"
account = "dmx"
client = "claude"

[profiles."claude-opus-4-8-thinking".models]
claude = "claude-opus-4-8-thinking"

[profiles."claude-fable-5"]
label = "Claude Fable"
account = "dmx"
client = "claude"

[profiles."claude-fable-5".models]
claude = "claude-fable-5"
```

验证清单：

```bash
aigw config import team-profiles.toml
aigw config export
```

清单只包含 Account、模型 Profile、URL、协议能力、模型名和推荐默认路由；不包含 Token、个人路由、Adapter 状态或本机路径。

## Release artifacts

每个 Release 应包含：

| 平台 | 原生包 | 便携包 |
|---|---|---|
| macOS Universal | `aigw_<version>_darwin_universal.pkg` | `darwin_amd64.tar.gz`, `darwin_arm64.tar.gz` |
| Linux x86-64 | `linux_amd64.deb`, `linux_amd64.rpm` | `linux_amd64.tar.gz` |
| Linux ARM64 | `linux_arm64.deb`, `linux_arm64.rpm` | `linux_arm64.tar.gz` |
| Windows x86-64 | `windows_amd64.msi` | `windows_amd64.zip` |
| Windows ARM64 | `windows_arm64.msi` | `windows_arm64.zip` |

还必须包含 `checksums.txt` 和 `aigw_<version>.spdx.json`。`amd64` 是常见 Intel/AMD 64 位 x86；`arm64` 是 ARM 64 位。

## Team member

推荐路径：下载对应原生安装包，校验 `checksums.txt` 后安装，然后运行：

```bash
aigw setup
aigw check
aigw balance dmx
```

团队清单路径：

```bash
aigw config import team-profiles.toml
aigw rotate dmx
aigw use gpt-5.6-sol-cdx --for codex
aigw use claude-opus-4-8-thinking --for claude
aigw check
```

需要在变更、升级或客户端故障后取得真实链路证据时，由使用者明确执行：

```bash
aigw verify --for all
```

该命令会调用 Claude 与 Codex 各一次最小模型请求，因此会消耗额度；它在两个 Adapter 投影和两条响应均通过后才保存不含 Token 的本机验证检查点。若随后某次配置变更需要撤销，使用：

```bash
aigw rollback                 # 最近一次完整验证检查点
aigw rollback --last-change   # 仅紧邻的一次配置备份
```

验证和回退均不会启动、停止、重启或重载 Claude/Codex 客户端。

只启用本机实际使用的客户端：

```bash
aigw adapter discover
aigw adapter enable claude --executable /absolute/path/to/claude
aigw adapter enable codex \
  --executable /absolute/path/to/codex \
  --target "$HOME/.codex/config.toml"
```

AIGW 的 Claude shim 位于用户级 shim 目录，例如 `~/.local/bin/claude` 或 `%LOCALAPPDATA%\Programs\aigw\bin\claude.cmd`。它只由 AIGW 管理，不覆盖外部 Claude，不写入 Codex 目录。若 PATH 中已有外部 `claude` 优先，`aigw doctor` 会提示修复；用户无需手工修改多个配置文件。

原生包安装与卸载只管理自己的程序文件，绝不会遍历用户目录来创建或删除 shim。若需删除 shim，由该用户执行 `aigw adapter disable claude`。

## Update

`aigw update` 根据安装渠道分流：

- 便携安装：下载归档、校验、原子替换当前二进制。
- macOS `.pkg`：下载并打开新版安装包。
- Linux `.deb/.rpm`：下载并调用系统包管理器安装。
- Windows `.msi`：下载并启动 Windows Installer。

## CI

CI 使用只读环境密钥后端：

```bash
export AIGW_SECRET_BACKEND=env
export AIGW_TOKEN_DMX=masked-ci-token
```

环境变量后端只读；`add`、`rotate`、rename 和 secret deletion 会失败，不会生成明文凭据文件。
