# AIGW CLI

AIGW 是面向团队的跨平台第三方 AI API 配置工具：统一管理 Account、模型 Profile、系统密钥、Claude/Codex 路由与客户端适配，**不运行后台服务，不承载 API 流量**。

```text
AIGW  当前状态
────────────────────────────────────────
配置
  当前服务       DMXAPI
  Profile        dmx

客户端
  ✓ Claude       继承默认服务 · 已就绪
  ✓ Codex        继承默认服务 · 已就绪

下一步
  aigw check
```

## 为什么需要 AIGW

- 团队可以共享网关 URL、协议和推荐路由，而不共享 Token。
- 一个 Account 对应一个服务商账户和一个系统密钥槽位；多个模型 Profile 可以继承同一个 Account。
- Claude 和 Codex 各自在自己的 Adapter 边界完成映射，互不污染目录。
- 切换、轮换、状态、余额和诊断只有一套命令，不再手工修改多处配置。

DMXAPI 只是一个 Account 示例；`gpt-5.6`、`gpt-5.5`、`gpt-5.5-ssvip`、`claude-sonnet`、`claude-opus`、`claude-fable` 是内置模型 Profile 示例。模型名对 AIGW 是透明字符串，团队清单可以继续增删。

## 安装

从私有 GitLab Release 下载与你平台匹配的安装包，并校验 `checksums.txt`。

| 平台 | 推荐安装包 | 便携包 |
|---|---|---|
| macOS Intel/Apple Silicon | `aigw_<version>_darwin_universal.pkg` | `darwin_amd64.tar.gz` / `darwin_arm64.tar.gz` |
| Linux x86-64 | `linux_amd64.deb` 或 `linux_amd64.rpm` | `linux_amd64.tar.gz` |
| Linux ARM64 | `linux_arm64.deb` 或 `linux_arm64.rpm` | `linux_arm64.tar.gz` |
| Windows x86-64 | `windows_amd64.msi` | `windows_amd64.zip` |
| Windows ARM64 | `windows_arm64.msi` | `windows_arm64.zip` |

`amd64` 表示常见 Intel/AMD 64 位 x86 机器；`arm64` 表示 Apple Silicon、ARM Linux 或 Windows on ARM。

也可以使用便携安装脚本：

```bash
sh install.sh
```

```powershell
.\install.ps1
```

便携安装默认放到 Unix 的 `~/.local/bin` 或 Windows 的 `%LOCALAPPDATA%\Programs\aigw\bin`。原生安装包负责系统安装语义；AIGW 不注册 daemon、launchd、systemd 或计划任务。

## 第一次使用

最简单路径：

```bash
aigw setup
```

它会引导选择团队 Profile、隐藏输入 Token、保存系统密钥、发现 Claude/Codex、写入各自 Adapter，并执行一次连通性检查。

团队清单模式：

```bash
aigw config import team-profiles.toml
aigw rotate dmx
aigw use gpt-5.6 --for codex
aigw use claude-opus --for claude
aigw check
aigw balance dmx
```

没有团队清单时：

```bash
aigw setup \
  --profile gateway \
  --label "Team Gateway" \
  --openai-url https://gateway.example/v1 \
  --anthropic-url https://gateway.example
```

Token 使用隐藏输入；自动化场景可将一行 Token 管道输入并添加 `--token-stdin`。

## 日常命令

```bash
aigw                         # 当前状态；首次运行会进入向导
aigw setup                   # 傻瓜式首次配置
aigw use [profile]           # 切换模型 Profile，可交互选择
aigw rotate [account]        # 更新 Account Token
aigw check                   # 配置、Token、客户端与网关健康检查
aigw balance [profile]       # 余额和 Token 额度；支持的服务商才显示精确值
aigw repair                  # 自动发现并修复 Adapter 漂移
aigw update                  # 按安装渠道更新
```

低频操作位于 `profile`、`route`、`adapter`、`account` 和 `config` 命名空间。运行 `aigw completion --help` 可生成 Bash、Zsh、Fish 或 PowerShell 补全。


## Account 与模型 Profile

AIGW 分两层管理：

- **Account**：上游服务商账户、URL、Token 和余额诊断，例如 `dmx`。
- **Profile**：用户日常切换的模型运行配置，例如 `gpt-5.6`、`gpt-5.5-ssvip`、`claude-opus`。

多个 Profile 可以引用同一个 Account，所以轮换 Token 只需要：

```bash
aigw rotate dmx
```

切换模型只需要：

```bash
aigw use gpt-5.6 --for codex
aigw use claude-fable --for claude
```

## 客户端边界

```bash
aigw adapter discover
aigw adapter enable claude --executable /absolute/path/to/claude
aigw adapter enable codex \
  --executable /absolute/path/to/codex \
  --target "$HOME/.codex/config.toml"
```

Claude shim 由 AIGW 放在用户级 AIGW shim 目录，例如 `~/.local/bin/claude`；它不写入 `~/.codex`，也不会覆盖非 AIGW-owned 的 `claude`。Claude Token 只在启动目标进程时映射为 `ANTHROPIC_AUTH_TOKEN`。

Codex 只接收带 AIGW 标记的 provider 投影，并通过官方 `login --with-api-key` 从 stdin 刷新认证。

## 诊断

`aigw check` / `aigw doctor` 会给出可操作判断：Token 无效、Token 被禁用、余额/额度耗尽、账号限制、限速、模型/渠道不可用、网关 5xx、网络/TLS/代理问题、本地密钥缺失、Adapter 漂移等。

支持精确账户诊断的服务商可绑定平台凭据：

```bash
aigw account connect dmx
aigw balance dmx
```

平台凭据独立保存在系统密钥存储中，不进入配置文件。

## 文档

- [核心概念](docs/concepts.md)
- [安全模型](docs/security.md)
- [团队推广](docs/team-rollout.md)
- [旧本机原型迁移](docs/migration.md)
- [产品设计](docs/design/2026-07-10-aigw-cli-product-design.md)

## 开发与验证

```bash
go test -race ./...
go vet ./...
sh scripts/package.sh 0.1.0-rc.1 dist
```

项目目标为 macOS、Linux、Windows 的 amd64 与 arm64。AIGW 只管理本地配置、密钥、路由和客户端 Adapter；任何传输代理或数据面网关均应作为独立项目维护。
