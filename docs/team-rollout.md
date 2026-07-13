# Team rollout

## Maintainer

维护一个不含 Token 的团队 Account + Profile 清单，例如 [`examples/team-profiles.toml`](../examples/team-profiles.toml)：

```toml
version = 2
recommended_default = "gpt-5.6-terra-cdx"

[accounts."team-gateway"]
label = "Team Gateway"

[accounts."team-gateway".endpoints]
openai_responses = "https://gateway.example/v1"
anthropic = "https://gateway.example"

[profiles."gpt-5.6-terra-cdx"]
label = "GPT-5.6 Terra Codex"
purpose = "Codex 代码与工程"
account = "team-gateway"
client = "codex"

[profiles."gpt-5.6-terra-cdx".models]
codex = "gpt-5.6-terra-cdx"

[profiles."claude-fable-5"]
label = "Claude Fable (recommended)"
purpose = "默认 Agent"
account = "team-gateway"
client = "claude"

[profiles."claude-fable-5".models]
claude = "claude-fable-5"

[profiles."claude-opus-4-8-thinking"]
label = "Claude Opus"
purpose = "复杂推理（按需）"
account = "team-gateway"
client = "claude"

[profiles."claude-opus-4-8-thinking".models]
claude = "claude-opus-4-8-thinking"

[profiles."claude-sonnet-5"]
label = "Claude Sonnet"
purpose = "平衡备选（按需）"
account = "team-gateway"
client = "claude"

[profiles."claude-sonnet-5".models]
claude = "claude-sonnet-5"
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

发布流水线在上传前必须通过 15 个工件的完整矩阵检查：macOS Universal pkg、六个便携包、四个 Linux 原生包、两个 Windows MSI、SBOM 与统一校验和。`checksums.txt` 必须恰好包含每个非 checksum 工件的一条 SHA-256 记录：无缺失、重复、额外或歧义路径。缺少平台专属构建工具或签名材料时，流水线必须失败而不是发布残缺资产。

维护者将 `package` job 调度到受管 macOS runner（tag：`macos`）；该 runner 必须具备 Go、`lipo`、`pkgbuild`、`productbuild`、`nfpm`、`wixl`、`uuidgen`、`msibuild`、`file`、`tar`、`zip`、`unzip`、`ar`、`bsdtar`、`msiextract`、`msiinfo`、`pkgutil`，以及 `sha256sum` 或 `shasum` 至少一个。job 会先运行 `check-package-runner.sh`，任一构建、验收或 SHA-256 能力缺失即失败；不能静默跳过工件，也不应回退到通用 Linux runner 产出残缺 Release。随后才构建完整矩阵、逐项重算 SHA-256，并检查所有便携包、macOS Universal pkg、Linux `.deb/.rpm` 和 Windows MSI 的载荷、CPU 架构及 MSI 平台 template。

还必须包含 `checksums.txt` 和 `aigw_<version>.spdx.json`。`amd64` 是常见 Intel/AMD 64 位 x86；`arm64` 是 ARM 64 位。Windows MSI 的 `ProductCode` 每次构建可以变化，但 `AigwExe` 与 `AigwPath` 的组件 GUID 按目标架构固定，以保证同一架构的 Major Upgrade 正确替换程序与用户 PATH 组件。MSI 的三段 `ProductVersion` 使用受测映射：第三段编码 SemVer patch 与预发布阶段，确保 `alpha < beta < rc < GA`，且相邻 patch 仍严格递增；不要把 RC、快照和 GA 压成同一个 MSI 版本。独立的 Linux 原生安装验收应在 Debian 与 RPM 系发行版 runner 上进行；兼容性容器的 `dpkg`/`rpm` 安装结果不能替代该原生发行版证据。

## Team member

推荐路径：下载对应原生安装包，校验 `checksums.txt` 后安装，然后运行：

```bash
aigw setup
aigw check
```

团队清单路径：

```bash
aigw config import team-profiles.toml
aigw rotate team-gateway
aigw catalog                 # 默认显示已配置模型与数量摘要
aigw catalog --all           # 显式展开完整模型目录
aigw use gpt-5.6-terra-cdx --for codex
aigw use claude-fable-5 --for claude
aigw check
```

团队清单和本机配置固定采用 schema v2。AIGW 只接受这一当前结构，避免迁移逻辑和并行配置路径；导入不会同步、认证、启动或重启任何客户端。

导入默认保护成员既有的 Account 身份。若清单中的同名 Account 或 Profile 与本机内容不一致，AIGW 会在写入前拒绝，避免成员已有 Token 被静默导向新的 URL。维护者更新清单后，成员应先核验差异；确认替换时再显式执行：

```sh
aigw config import team-profiles.toml --replace-account team-gateway
# 如 Profile 同名且模型或用途也需要替换：
aigw config import team-profiles.toml --replace-profile gpt-5.6-terra-cdx
```

`--replace-account` 不会复制、删除或更改成员系统密钥中的 Token；它只接受经成员确认的 Account 元数据替换。

若某个已导入 Account 的目录中出现尚未配置的模型，成员可在不复制 Token 的前提下添加一个本机 Profile：

```bash
aigw profile add gpt-next --account team-gateway --for codex --model gpt-next --purpose "已验证的代码任务"
aigw use gpt-next --for codex
```

维护者应先审核模型的协议、权限、价格与适用任务；`catalog` 的发现结果不构成模型准入或自动路由。
新客户端还必须通过 [Adapter 准入](adapter-admission.md)：有独立配置边界、密钥证明、协议与回退证据后，才可进入团队清单。

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

AIGW 的 Claude shim 位于专属目录：macOS 为 `~/Library/Application Support/aigw/bin/claude`，Linux 为 `${XDG_DATA_HOME:-~/.local/share}/aigw/bin/claude`，Windows 为 `%LOCALAPPDATA%\Programs\aigw\bin\claude.cmd`。它只由 AIGW 管理，不覆盖外部 Claude，不写入 Codex 目录。启用时 AIGW 为当前用户写入一个带边界标记的无密钥 PATH 块；用户无需手工修改多个配置文件。

原生包安装与卸载只管理自己的程序文件，绝不会遍历用户目录来创建或删除 shim。若需删除 shim，由该用户执行 `aigw adapter disable claude`。

AIGW 不部署网关，也不占用本地端口。若组织未来部署独立网关，AIGW 只把它视为 HTTPS Account 端点，不代管其进程。

## Update

`aigw update` 根据安装渠道分流：

- 便携安装：下载归档、校验、原子替换当前二进制。
- macOS `.pkg`：下载并打开新版安装包。
- Linux `.deb/.rpm`：下载并调用系统包管理器安装。
- Windows `.msi`：下载并启动 Windows Installer。

私有 GitLab Release 优先使用已认证的 `glab`，并继承
`AIGW_GL_HOST`。若成员未安装 `glab`，可仅在当前终端提供
`GITLAB_TOKEN` 后执行同一条 `aigw update`；Token 不会被写入 AIGW
配置、命令行或错误信息。所有安装渠道都会先校验发布工件的 SHA-256。

## CI

CI 使用只读环境密钥后端：

```bash
export AIGW_SECRET_BACKEND=env
export AIGW_TOKEN_TEAM_GATEWAY=masked-ci-token
```

环境变量后端只读；`add`、`rotate`、rename 和 secret deletion 会失败，不会生成明文凭据文件。
