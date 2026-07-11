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

DMXAPI 只是一个 Account 示例；`gpt-5.6-sol-cdx`、`gpt-5.5`、`gpt-5.5-ssvip`、`claude-sonnet-5`、`claude-opus-4-8-thinking`、`claude-fable-5` 是内置模型 Profile 示例。模型名对 AIGW 是透明字符串，团队清单可以继续增删。

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

它会引导选择模型 Profile、隐藏输入 Account Token、保存系统密钥、发现 Claude/Codex、写入各自 Adapter，并执行一次连通性检查。

团队清单模式：

```bash
aigw config import team-profiles.toml
aigw rotate dmx
aigw use gpt-5.6-sol-cdx --for codex
aigw use claude-opus-4-8-thinking --for claude
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
aigw balance [account]       # 余额和 Token 额度；支持的服务商才显示精确值
aigw sync                    # 仅对齐客户端配置；不重启客户端、不改动认证
aigw adapter auth codex      # 仅重新绑定当前 Account 的 Codex 原生认证
aigw repair                  # 自动发现并修复 Adapter 漂移
aigw verify --for all        # 明确执行两次最小真实模型请求，并建立验证检查点
aigw rollback                # 回退到最近一次完整验证配置，不重启客户端
aigw update                  # 按安装渠道更新
```

低频操作位于 `profile`、`route`、`adapter`、`account` 和 `config` 命名空间。运行 `aigw completion --help` 可生成 Bash、Zsh、Fish 或 PowerShell 补全。


## Account 与模型 Profile

AIGW 分两层管理：

- **Account**：上游服务商账户、URL、Token 和余额诊断，例如 `dmx`。
- **Profile**：用户日常切换的模型运行配置，例如 `gpt-5.6-sol-cdx`、`gpt-5.5-ssvip`、`claude-opus-4-8-thinking`。

多个 Profile 可以引用同一个 Account，所以轮换 Token 只需要：

```bash
aigw rotate dmx
```

切换模型只需要：

```bash
aigw use gpt-5.6-sol-cdx --for codex
aigw use claude-fable-5 --for claude
```

## 客户端边界

```bash
aigw adapter discover
aigw adapter enable claude --executable /absolute/path/to/claude
aigw adapter enable codex \
  --executable /absolute/path/to/codex \
  --target "$HOME/.codex/config.toml"
```

Claude shim 由 AIGW 放在**专属数据目录**：macOS 为 `~/Library/Application Support/aigw/bin/claude`，Linux 为 `${XDG_DATA_HOME:-~/.local/share}/aigw/bin/claude`，Windows 为 `%LOCALAPPDATA%\Programs\aigw\bin\claude.cmd`。启用 Claude Adapter 时，AIGW 只在当前用户的 shell 配置写入一个有边界标记、无密钥的 PATH 块；它不写入 `~/.codex`，也不会覆盖非 AIGW-owned 的 `claude`。Claude Token 只在启动目标进程时映射为 `ANTHROPIC_AUTH_TOKEN`。

Codex 只接收带 AIGW 标记的顶层 `model`、`model_provider` 和 provider 投影。`aigw doctor` 会检查这三处是否仍与当前 Profile 一致；发现手工漂移时，用 `aigw sync` 只恢复配置，不会启动、关闭或重启 Codex。

同一 Account 内切换模型只更新配置，不重复绑定凭据。首次启用 Codex、切换到另一个 Account、`aigw rotate` 或显式运行 `aigw adapter auth codex` 时，AIGW 才通过官方 `login --with-api-key` 从 stdin 执行一次有 20 秒上限的认证绑定。

## 验证与回退

`aigw test` 是无模型调用的连通性与认证检查；`aigw verify --for claude|codex` 则会发送一次有上限的真实模型请求，并要求返回 `AIGW_OK`。它只适合在用户明确允许消耗一次最小额度时使用。

`aigw verify --for all` 还会先确认 Claude shim、Codex 可执行文件和所有 Codex 投影均就绪；仅两条真实链路均通过后，才在本机保存**不含密钥**的完整验证检查点。`aigw rollback` 优先恢复该检查点；`aigw rollback --last-change` 只恢复紧邻的配置备份。二者只恢复 AIGW 管理的配置投影，绝不启动、停止、重启或重载 Claude/Codex 客户端。

## 诊断

`aigw check` / `aigw doctor` 会给出可操作判断：Token 无效、Token 被禁用、余额/额度耗尽、账号限制、限速、模型/渠道不可用、网关 5xx、网络/TLS/代理问题、本地密钥缺失、Codex 模型或 provider 投影漂移等。

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

## 卸载边界

便携安装的卸载脚本只移除当前用户安装目录中的 `aigw` 与 AIGW-owned Claude shim。原生 `.pkg`、`.deb`、`.rpm`、`.msi` 安装器只管理自身安装的程序文件，**不会遍历用户目录或删除任何用户 shim**；在卸载前如需清理 shim，请以该用户身份运行 `aigw adapter disable claude`。配置、系统密钥和客户端用户配置始终保留，供重新安装或受控离网使用。

## Proxy 边界

默认不需要本地 proxy，也不监听 8791、8888 或任何端口：Claude/Codex 通过各自 Adapter 直连 Account 的 HTTPS 上游端点。仅当上游协议不兼容、必须进行响应格式转换/集中审计，或组织网络强制要求出口代理时，才应另行部署一个独立的数据面 proxy；其端口、运行方式和生命周期不由 AIGW 管理。

若确需部署该独立 proxy，端口必须由该项目显式配置并做冲突检查；`8888` 不是约定默认值，且常与调试代理或企业工具冲突。AIGW 只接收该 proxy 的明确 URL（例如 `http://127.0.0.1:<chosen-port>/v1`），不会启动、停止、探测或重启它。

## 开发与验证

```bash
go test -race ./...
go vet ./...
sh scripts/package.sh 0.1.0-rc.1 dist
```

项目目标为 macOS、Linux、Windows 的 amd64 与 arm64。AIGW 只管理本地配置、密钥、路由和客户端 Adapter；任何传输代理或数据面网关均应作为独立项目维护。
