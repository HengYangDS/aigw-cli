# AIGW CLI

AIGW 是**本机优先**、可供团队分发的跨平台第三方 AI API 配置工具：统一管理 Account、模型 Profile、系统密钥、Claude/Codex 路由与客户端适配，**不运行后台服务，不承载 API 流量**。

```text
AIGW  当前状态
────────────────────────────────────────
配置
  当前 Account      Team Gateway
  默认 Profile      gpt-5.6-terra

客户端
  ✓ Claude       claude-fable-5 · 单独指定 · 已就绪
  ✓ Codex        gpt-5.6-terra · 继承默认 · 已就绪

下一步
  aigw check
```

## 为什么需要 AIGW

- 个人在本机即可完整使用：一个二进制、系统密钥存储、多个服务 Account、多个模型 Profile、显式客户端 Route 和各自独立的 Claude/Codex Adapter；不依赖团队后台、内网或本地监听端口。
- 团队可以共享网关 URL、协议和推荐路由，而不共享 Token。
- 一个 Account 对应一个服务商账户和一个系统密钥槽位；多个模型 Profile 可以继承同一个 Account。
- Claude 和 Codex 各自在自己的 Adapter 边界完成映射，互不污染目录。
- 切换、轮换、状态、余额和诊断只有一套命令，不再手工修改多处配置。

本机直连不是“只能连一个服务”：每个服务 Account 有自己的一把系统密钥和可用协议端点；同一 Account 下可有任意多个模型 Profile，并由 Claude、Codex 或默认 Route 显式选择。团队集中 Gateway 不是安装或日常使用的前提。只有组织确实需要集中审计、预算、协议转换或统一出口时，才应单独评测和部署一个独立 Gateway；AIGW 只把它当作普通 HTTPS Account 入口，绝不管理其端口、进程或上游密钥。

任意上游服务商都可作为一个 Account。当前精简的可运行基线是 `gpt-5.6-terra`（Codex）和 `claude-fable-5`（默认 Agent）；Sonnet 与 Opus 仅作明确的按需选择。示例团队清单用可选的 `purpose` 标出每个 Profile 的日常用途，帮助成员选择，而不改变路由或密钥边界。模型名对 AIGW 是透明字符串，团队可按自身已验证的能力增删；[模型策略](docs/model-strategy.md)定义了推荐集与新客户端的准入边界。

## 安装

从私有 GitLab Release 下载与你平台匹配的安装包，并校验 `checksums.txt`。

正式团队 Release 在上传前必须通过完整 15 工件矩阵；缺失某平台的原生构建工具时，流水线会拒绝发布，不会悄悄生成残缺 Release。校验和与 SBOM 是当前 RC 的可验证交付物。无预发布后缀的 GA tag 在组织的 macOS/Windows 签名、公证与验证作业落地前会被 CI 明确阻断，不能伪造或绕过；见[发布证据契约](docs/release-readiness.md)。

| 平台 | 推荐安装包 | 便携包 |
|---|---|---|
| macOS（Intel 或 Apple Silicon） | `aigw_<version>_darwin_universal.pkg` | 按芯片选择下列便携包 |
| macOS Intel | 同上（Universal `.pkg`） | `aigw_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | 同上（Universal `.pkg`） | `aigw_<version>_darwin_arm64.tar.gz` |
| Linux x86-64 | `linux_amd64.deb` 或 `linux_amd64.rpm` | `linux_amd64.tar.gz` |
| Linux ARM64 | `linux_arm64.deb` 或 `linux_arm64.rpm` | `linux_arm64.tar.gz` |
| Windows x86-64 | `windows_amd64.msi` | `windows_amd64.zip` |
| Windows ARM64 | `windows_arm64.msi` | `windows_arm64.zip` |

`darwin_universal.pkg` 内含 Intel（`amd64`）和 Apple Silicon（`arm64`）两个原生架构，安装时自动选择；它不是 ARM 专用包。`amd64` 表示常见 Intel/AMD 64 位 x86 机器；`arm64` 表示 Apple Silicon、ARM Linux 或 Windows on ARM。

解压对应便携包后，在包目录使用便携安装脚本：

```bash
sh install.sh
```

```powershell
.\install.ps1
```

脚本只安装包内的二进制，不访问网络、不读取 GitLab Token；安装完成后的升级统一使用 `aigw update`。便携安装默认放到 Unix 的 `~/.local/bin` 或 Windows 的 `%LOCALAPPDATA%\Programs\aigw\bin`。Unix 安装器即使从受限 `PATH` 启动也只使用系统基础工具；仅在 zsh 的原始 `PATH` 缺少系统目录时，才创建一个 AIGW-owned、无密钥、可卸载的 `.zshenv` bootstrap。原生安装包负责系统安装语义；AIGW 不注册 daemon、launchd、systemd 或计划任务。

## 第一次使用

最简单路径：

```bash
aigw setup
```

首次运行 `aigw` 或空参数 `aigw setup` 会引导输入 Account、首个模型 Profile、客户端、端点和隐藏 Token；它不预设任何服务商、URL、Token 槽位或模型。随后会发现适用的 Claude/Codex Adapter，并执行一次连通性检查。

团队清单模式：

```bash
aigw config import team-profiles.toml
aigw rotate team-gateway
aigw use gpt-5.6-terra --for codex
aigw use claude-fable-5 --for claude
aigw check
```

团队清单与本机配置统一使用 schema v2；AIGW 仅接受这一当前结构，避免迁移逻辑和并行配置路径。导入、轮换和切换都不会同步、认证、启动、关闭或重启 Claude/Codex。

团队清单不能静默接管本机身份：同名 Account 或 Profile 的内容完全一致时导入幂等；若端点、协议或模型等内容不同，导入会拒绝，以免把既有系统 Token 指向新地址。经人工核验后，才可显式替换指定对象：

```sh
aigw config import team-profiles.toml --replace-account team-gateway
aigw config import team-profiles.toml --replace-profile gpt-5.6-terra
```

`--replace-account` 仅替换 Account 元数据；对应的系统密钥槽位与 Token 不会被团队清单读取、写入或删除。

没有团队清单时：

```bash
aigw setup \
  --account team-gateway \
  --profile gpt-5.6-terra \
  --label "Team Gateway" \
  --openai-url https://gateway.example/v1 \
  --for codex \
  --model gpt-5.6-terra
```

Token 使用隐藏输入；自动化场景可将一行 Token 管道输入并添加 `--token-stdin`。

## 日常命令

```bash
aigw                         # 当前状态；首次运行会进入向导
aigw setup                   # 傻瓜式首次配置
aigw use [profile]           # 切换模型 Profile，可交互选择
aigw profile add ... --purpose "代码与工程" # 为 Profile 添加一行用途提示
aigw rotate [account]        # 更新 Account Token
aigw catalog [--all|--json]  # 默认紧凑模型摘要；显式查看完整目录或 JSON
aigw check                   # 配置、Token、客户端与网关健康检查
aigw balance [account]       # 仅对显式配置且本版本支持的服务商显示精确值
aigw sync                    # 仅对齐客户端配置；不重启客户端、不改动认证
aigw adapter auth codex      # 仅重新绑定当前 Account 的 Codex 原生认证
aigw repair                  # 自动发现并修复 Adapter 漂移
aigw verify --for all        # 明确执行两次最小真实模型请求，并建立验证检查点
aigw rollback                # 回退到最近一次完整验证配置，不重启客户端
aigw update                  # 按安装渠道更新
aigw update --rollback       # 仅回退便携版 AIGW 程序，不访问网络
```

低频操作位于 `profile`、`route`、`adapter`、`account` 和 `config` 命名空间。运行 `aigw completion --help` 可生成 Bash、Zsh、Fish 或 PowerShell 补全。


## Account 与模型 Profile

AIGW 分两层管理：

- **Account**：上游服务商账户、URL、Token 和可选的服务商精确诊断，例如 `team-gateway`。
- **Profile**：用户日常切换的模型运行配置，例如 `gpt-5.6-terra`、`claude-fable-5`、`claude-opus-4-8-thinking`；可选的 `purpose` 仅提供一行用途提示。

Profile 与模型 ID 保持上游的 canonical 名称；客户端语义由 `client` 表达，不再以历史客户端后缀编码进 GPT 名称。

多个 Profile 可以引用同一个 Account，所以轮换 Token 只需要：

```bash
aigw rotate team-gateway
```

切换模型只需要：

```bash
aigw use gpt-5.6-terra --for codex
aigw use claude-fable-5 --for claude
```

添加第二个服务时使用一次 `aigw add <account>` 并录入该服务自己的 Token；在既有服务下添加模型时，不复制 URL 或 Token：

```bash
aigw account edit team-gateway --openai-url https://gateway.example/v1
aigw profile add gpt-next --account team-gateway --for codex --model gpt-next --purpose "已验证的代码任务"
aigw catalog
aigw use gpt-next --for codex
```

`catalog` 只读取该 Account 的 `/v1/models`；默认只显示已配置模型与数量摘要，`--all` 显式展开完整目录，`--json` 保持完整机器输出。列出的模型不自动成为 Profile，也不表示已经通过特定客户端、工具、视觉或推理能力验证。

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

`aigw update --rollback` 与上述配置回退完全独立：它只在便携安装中交换当前 AIGW
程序与唯一的上一版本副本，不访问网络、不读取 Token、不改动配置、密钥或客户端。
原生 `.pkg`、`.deb`、`.rpm`、`.msi` 安装应使用各自的包管理器回退。

## 诊断

`aigw check` / `aigw doctor` 会给出可操作判断：Token 无效、Token 被禁用、余额/额度耗尽、账号限制、限速、模型/渠道不可用、网关 5xx、网络/TLS/代理问题、本地密钥缺失、Codex 模型或 provider 投影漂移等。

`aigw check` 诊断当前默认 Profile 的 Account，并分别检查已启用客户端的本地路由与 Adapter；它不会把客户端 override 误当成默认服务，也不会静默扫描无关 Account。需要明确验证某个客户端端点时使用 `aigw test --for claude|codex`。只有团队清单显式配置、且当前 AIGW 版本包含对应 Provider Diagnostics 的服务商，才会显示精确余额入口；该平台凭据独立存储，不进入配置文件：

```bash
aigw account connect <account>
aigw balance <account>
```

若没有精确诊断驱动，`aigw balance` 会明确说明原因，不会影响 `check`、路由、Token 轮换或客户端使用。

## 文档

- [核心概念](docs/concepts.md)
- [安全模型](docs/security.md)
- [模型策略与客户端准入](docs/model-strategy.md)
- [Adapter 准入](docs/adapter-admission.md)
- [团队推广](docs/team-rollout.md)
- [产品设计](docs/design/2026-07-10-aigw-cli-product-design.md)

## 卸载边界

便携安装的卸载脚本只移除当前用户安装目录中的 `aigw` 与 AIGW-owned Claude shim。原生 `.pkg`、`.deb`、`.rpm`、`.msi` 安装器只管理自身安装的程序文件，**不会遍历用户目录或删除任何用户 shim**；在卸载前如需清理 shim，请以该用户身份运行 `aigw adapter disable claude`。配置、系统密钥和客户端用户配置始终保留，供重新安装或受控离网使用。

## 外部网关边界

AIGW 不运行 proxy，不监听任何端口。若组织未来部署独立网关，它对 AIGW 只是一个 HTTPS Account 端点；该网关的部署、生命周期与密钥均不属于 AIGW。

## 开发与验证

```bash
go test -race ./...
go vet ./...
sh scripts/check-retired-residue.sh
sh scripts/check-package-runner.sh
AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh 0.1.0-rc.1 dist
sh scripts/test-release-artifacts.sh
sh scripts/test-msi-version.sh
sh scripts/test-release-package-layout.sh dist 0.1.0-rc.1
# 可选：本地/容器兼容性验收；依赖 Docker，不能替代 Debian/Fedora runner 的原生证据。
sh scripts/test-linux-native-install.sh dist 0.1.0-rc.1
```

项目目标为 macOS、Linux、Windows 的 amd64 与 arm64。AIGW 只管理本地配置、密钥、路由和客户端 Adapter；任何传输代理或数据面网关均应作为独立项目维护。
