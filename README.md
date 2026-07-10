# AIGW CLI

面向团队的跨平台第三方 AI API 配置工具：统一管理 Provider Profile、系统密钥、Claude/Codex 路由与客户端适配，**不运行后台服务，不承载 API 流量**。

```text
Current
  Default   dmx
  Claude    dmx          inherited · ready
  Codex     dmx          inherited · ready

Profiles    1
Next        aigw test
```

## 为什么需要 AIGW

- 团队可以共享网关 URL、协议和推荐路由，而不共享 Token。
- 一个 Profile 对应一个服务商账户和一个系统密钥槽位。
- Claude 和 Codex 各自在自己的 Adapter 边界完成映射，互不污染目录。
- 切换、轮换、状态和测试只有一套命令，不再手工修改多处配置。

DMXAPI 只是一个 Profile。相同工具也可管理其他 OpenAI Responses / Anthropic 兼容网关。

## 安装

从私有 GitLab Release 下载对应归档，校验 `checksums.txt` 后运行归档内安装器：

```bash
sh install.sh
```

```powershell
.\install.ps1
```

也可以在已认证 `glab` 的工作站上固定版本安装：

```bash
AIGW_VERSION=v0.1.0 sh scripts/install.sh
```

默认安装到 Unix 的 `~/.local/bin` 或 Windows 的 `%LOCALAPPDATA%\Programs\aigw`。安装器不修改 shell 配置、不启用客户端，也不注册 daemon、launchd、systemd 或计划任务。

## 第一次使用

使用团队清单：

```bash
aigw config import team-profiles.toml
aigw rotate dmx
aigw use dmx --all
aigw test
aigw doctor
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
aigw                         # 当前状态
aigw add <profile>           # 添加 Profile
aigw use <profile>           # 切换默认 Profile
aigw use <profile> --for claude
aigw use <profile> --all     # 全部恢复继承并切换
aigw rotate <profile>        # 原子轮换密钥
aigw status [--json]
aigw test [--for claude|codex]
aigw doctor [--json]
aigw sync                    # 仅用于修复 Adapter 漂移
```

低频操作位于 `profile`、`route`、`adapter` 和 `config` 命名空间。运行 `aigw completion --help` 可生成 Bash、Zsh、Fish 或 PowerShell 补全。

## 客户端 Adapter

```bash
aigw adapter discover
aigw adapter enable claude --executable /absolute/path/to/claude
aigw adapter enable codex \
  --executable /absolute/path/to/codex \
  --target "$HOME/.codex/config.toml"
```

Claude Token 只在启动目标进程时映射为 `ANTHROPIC_AUTH_TOKEN`。Codex 只接收带 AIGW 标记的 provider 投影，并通过官方 `login --with-api-key` 从 stdin 刷新认证。

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
sh scripts/package.sh 0.1.0 dist
```

项目目标为 macOS、Linux、Windows 的 amd64 与 arm64。`codex-dmx-proxy` 是 subgroup 下独立维护的传输兼容项目，不属于 AIGW 核心生命周期。
