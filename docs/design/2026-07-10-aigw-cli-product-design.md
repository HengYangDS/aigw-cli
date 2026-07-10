# AIGW CLI 产品设计

**状态：** 已确认  
**产品名：** AIGW  
**命令：** `aigw`  
**仓库：** `dig/misc/agentic-third-party-api/aigw-cli`

## 1. 定位

AIGW 是面向团队的跨平台第三方 AI API 配置、密钥与客户端路由工具。它是一个按需执行的本地 CLI，不承载 API 流量，不运行后台服务，也不是中央密钥分发系统。

仓库名使用 `aigw-cli`，明确它是管理客户端而非真正的数据面网关。DMXAPI 只是一个可配置 Profile，不是产品身份。

## 2. 领域模型

### Profile

一个 Profile 表示一个服务商实例、它支持的协议端点及一个 Token。相同服务商的不同 Token 必须建立不同 Profile，例如 `dmx-main` 与 `dmx-backup`。

### Endpoint

Profile 可提供 `openai-responses` 和 `anthropic` 两类端点，它们共享该 Profile 的唯一 Token。端点是协议能力，不是客户端配置。

### Route

Route 将客户端使用面映射到 Profile。解析顺序为：单次命令的 `--profile`、客户端覆盖路由、默认路由。未设置客户端覆盖时自动继承默认路由，不保存重复副本。

### Adapter

Adapter 将解析后的 Profile 投影到一个客户端边界。Claude Adapter 只处理 Claude，Codex Adapter 只处理 Codex。任何 Claude 文件都不得进入 `~/.codex`，反之亦然。

## 3. 密钥治理

逻辑密钥地址统一为：

```text
service = AIGW_TOKEN
account = <profile-id>
```

默认后端：macOS Keychain、Windows Credential Manager、Linux Secret Service。CI 可显式选择只读环境变量后端。Linux 没有安全存储时必须清晰失败，不得静默写入明文文件。

禁止通过 `--token value` 传递密钥；只接受隐藏交互输入或 `--token-stdin`。任何结构化输出、诊断、备份和错误信息均不得出现 Token 或 Authorization Header。包含 userinfo 或疑似密钥查询参数的 URL 拒绝保存。

## 4. 客户端边界

Claude 通过独立 shim 启动，Token 仅在目标进程边界映射为 `ANTHROPIC_AUTH_TOKEN`，端点映射为 `ANTHROPIC_BASE_URL`。映射不持久化到 Claude 设置文件或 shell 启动文件。

Codex CLI 优先通过独立执行边界传递凭据。对无法经 shim 启动的 Codex Desktop，AIGW 使用 Codex 官方 `login --with-api-key` 边界刷新认证，并只维护带 AIGW 标记的 provider 配置。不得直接写入明文认证文件。

## 5. 用户体验

首次使用由 `aigw setup` 完成发现客户端、导入团队清单、创建 Profile、隐藏输入 Token、测试端点、设置路由和启用 Adapter。配置变更自动同步，用户不需要记住 `apply`；`aigw sync` 仅用于恢复漂移。

日常命令保持在一个屏幕内：

```text
aigw
aigw add <profile>
aigw use <profile> [--for claude|codex] [--all]
aigw rotate <profile>
aigw status [--json]
aigw test [--for claude|codex]
aigw doctor [--json]
aigw sync
```

高级能力收进 `profile`、`route`、`adapter`、`config` 和 `completion` 命名空间。不提供 `dmx-*`、`init-dmx`、`apply` 等旧兼容命令。错误必须同时给出原因、影响和一条可执行修复命令。

## 6. 配置与团队复用

本机配置使用 TOML，仅包含 Profile 元数据、端点、路由、Adapter 状态与可执行文件路径。团队清单可安全提交到仓库，包含 Profile 名称、URL、协议和推荐默认路由，但不包含 Token、个人覆盖路由、客户端登录状态或本机路径。

配置位置遵守平台约定：

- macOS：`~/Library/Application Support/aigw/config.toml`
- Linux：`${XDG_CONFIG_HOME:-~/.config}/aigw/config.toml`
- Windows：`%APPDATA%\aigw\config.toml`

所有配置写入使用锁、校验、备份、原子替换和失败回滚。Adapter 只恢复自己曾修改且未被用户再次更改的字段；冲突时停止而不是覆盖用户内容。

## 7. 技术与交付

采用 Go 构建单文件二进制，无 Python/Node 运行时要求。管理命令执行后退出；不安装 daemon、watchdog、launchd、systemd 或计划任务。Claude/Codex shim 只负责进程边界转发。

GitLab Release 为 macOS、Linux、Windows 的 amd64/arm64 生成压缩包、SHA-256 校验和与 SBOM。首版提供用户级 `install.sh` 和 `install.ps1`，支持固定版本安装与卸载。后续再增加 Homebrew、Scoop、deb/rpm。

## 8. 现有资产边界

`codex-dmx-proxy` 是独立的 DMX/Codex 传输兼容组件，依赖本地代理与 watchdog，不进入 AIGW 核心，也不由 AIGW 管理。AIGW 可使用用户配置的本地端点，但不知道代理生命周期。

本机 Python 原型仅作为迁移来源和行为参考。新版本完成验证后，迁移 Profile、Route 与现有 `AIGW_TOKEN/<profile>` 密钥，替换 shim，然后删除旧 Python 工具、旧文档和废弃命令；不保留兼容别名。

## 9. 验收标准

1. 六个目标 OS/架构均可交叉编译。
2. 配置及导出物不含 Token，测试覆盖泄漏防线。
3. Profile、继承路由、密钥后端和两个 Adapter 可独立测试。
4. `setup/add/use/rotate/status/test/doctor/sync` 形成完整日常闭环。
5. 团队清单可导入且不能携带密钥。
6. 安装、升级、卸载不覆盖非 AIGW-owned 用户配置。
7. 本机旧原型迁移后，`aigw doctor`、`aigw test` 和客户端边界验证通过。
