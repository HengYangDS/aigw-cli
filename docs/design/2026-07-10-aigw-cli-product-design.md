# AIGW CLI 产品设计

**状态：** 已确认  
**产品名：** AIGW  
**命令：** `aigw`  
**仓库：** `dig/misc/agentic-third-party-api/aigw-cli`

## 1. 定位

AIGW 是面向团队的跨平台第三方 AI API Account、模型 Profile、密钥与客户端路由工具。它是一个按需执行的本地 CLI，不承载 API 流量，不运行后台服务，也不是中央密钥分发系统。

仓库名使用 `aigw-cli`，明确它是管理客户端而非真正的数据面网关。DMXAPI 只是一个可配置 Account 示例，不是产品身份。

## 2. 领域模型

### Account

一个 Account 表示一个服务商实例、它支持的协议端点及一个 Token。相同服务商的不同 Token 必须建立不同 Account，例如 `primary-gateway` 与 `backup-gateway`。

### Runtime Profile

一个 Runtime Profile 表示用户日常切换的模型运行配置，引用一个 Account，并可限定客户端与模型名，例如 `gpt-5.6-sol-cdx`、`gpt-5.5-ssvip`、`claude-opus-4-8-thinking`、`claude-fable-5`。模型名对 AIGW 是透明字符串，由上游网关解释。

### Endpoint

Account 可提供 `openai-responses` 和 `anthropic` 两类端点，它们共享该 Account 的唯一 Token。端点是协议能力，不是客户端配置。Runtime Profile 只选择模型和客户端作用面。

### Route

Route 将客户端使用面映射到 Runtime Profile。解析顺序为：单次命令的 `--profile`、客户端覆盖路由、默认路由。未设置客户端覆盖时自动继承默认路由，不保存重复副本。

### Adapter

Adapter 将解析后的 Profile 投影到一个客户端边界。Claude Adapter 只处理 Claude，Codex Adapter 只处理 Codex。任何 Claude 文件都不得进入 `~/.codex`，反之亦然。

## 3. 密钥治理

逻辑密钥地址统一为：

```text
service = AIGW_TOKEN
account = <account-id>
```

默认后端：macOS Keychain、Windows Credential Manager、Linux Secret Service。CI 可显式选择只读环境变量后端。Linux 没有安全存储时必须清晰失败，不得静默写入明文文件。

禁止通过 `--token value` 传递密钥；只接受隐藏交互输入或 `--token-stdin`。任何结构化输出、诊断、备份和错误信息均不得出现 Token 或 Authorization Header。包含 userinfo 或疑似密钥查询参数的 URL 拒绝保存。

## 4. 客户端边界

Claude 通过独立 shim 启动，Token 仅在目标进程边界映射为 `ANTHROPIC_AUTH_TOKEN`，端点映射为 `ANTHROPIC_BASE_URL`。映射不持久化到 Claude 设置文件或 shell 启动文件。

Codex CLI 优先通过独立执行边界传递凭据。对无法经 shim 启动的 Codex Desktop，AIGW 只在首次启用、Account 切换、Token 轮换或显式 `aigw adapter auth codex` 时通过 Codex 官方 `login --with-api-key` 边界刷新认证；模型切换和 `aigw sync` 不触发登录。AIGW 只维护带 AIGW 标记的顶层 `model`、`model_provider` 与 provider 配置，不得直接写入明文认证文件，也不得启动、停止或重启桌面客户端。

## 5. 用户体验

首次使用由 `aigw setup` 完成发现客户端、导入团队清单、选择模型 Profile、隐藏输入 Account Token、测试端点、设置路由和启用 Adapter。配置变更自动同步；同一 Account 内的模型切换不重复绑定认证；`aigw sync` 仅用于恢复漂移，并且不改变认证或客户端进程状态。

日常命令保持在一个屏幕内：

```text
aigw
aigw add <account>
aigw use <profile> [--for claude|codex] [--all]
aigw rotate <account>
aigw status [--json]
aigw test [--for claude|codex]
aigw verify --for claude|codex|all
aigw rollback [--last-change]
aigw doctor [--json]
aigw sync
```

`test` 只检查端点和认证；`verify` 是显式付费动作，发送一次最小真实模型请求并要求精确回显 `AIGW_OK`。`verify --for all` 仅在 Claude shim、Codex 投影和两条真实协议链路都通过后保存不含密钥的验证检查点。`rollback` 优先恢复该检查点，`--last-change` 仅恢复上一份配置备份；两者都不会控制客户端生命周期。

高级能力收进 `profile`、`route`、`adapter`、`config` 和 `completion` 命名空间。不提供旧的服务商专用兼容命令或别名。错误必须同时给出原因、影响和一条可执行修复命令。

## 6. 配置与团队复用

本机配置使用 TOML，仅包含 Account 元数据、端点、Runtime Profile、模型名、路由、Adapter 状态与可执行文件路径。团队清单可安全提交到仓库，包含 Account、Profile、URL、协议、模型名和推荐默认路由，但不包含 Token、个人覆盖路由、客户端登录状态或本机路径。

配置位置遵守平台约定：

- macOS：`~/Library/Application Support/aigw/config.toml`
- Linux：`${XDG_CONFIG_HOME:-~/.config}/aigw/config.toml`
- Windows：`%APPDATA%\aigw\config.toml`

所有配置写入使用锁、校验、备份、原子替换和失败回滚。Adapter 只恢复自己曾修改且未被用户再次更改的字段；冲突时停止而不是覆盖用户内容。

## 7. 技术与交付

采用 Go 构建单文件二进制，无 Python/Node 运行时要求。管理命令执行后退出；不安装 daemon、watchdog、launchd、systemd 或计划任务。Claude/Codex shim 只负责进程边界转发。Claude shim 位于用户级 AIGW shim 目录，不能放入 `~/.codex`，也不能写入包管理器拥有的系统目录。

GitLab Release 为 macOS、Linux、Windows 的 amd64/arm64 生成 portable 压缩包、原生安装包、SHA-256 校验和与 SBOM。原生安装包包括 macOS Universal `.pkg`、Linux amd64/arm64 `.deb` 与 `.rpm`、Windows amd64/arm64 `.msi`。`install.sh` 和 `install.ps1` 作为便携安装兜底，支持固定版本安装与卸载。包安装与卸载只管理包拥有的文件，绝不遍历用户目录、创建或删除任何用户 shim；shim 始终由该用户运行的 AIGW Adapter 命令管理。

## 8. 现有资产边界

传输代理、兼容转发器和数据面网关是独立项目，不进入 AIGW 核心，也不由 AIGW 管理。AIGW 可使用用户配置的本地端点，但不知道代理生命周期。

本机 Python 原型仅作为迁移来源和行为参考。新版本完成验证后，迁移 Account、Runtime Profile、Route 与现有 `AIGW_TOKEN/<account>` 密钥，替换 shim，然后删除旧 Python 工具、旧文档和废弃命令；不保留兼容别名。

## 9. 验收标准

1. 六个目标 OS/架构均可交叉编译。
2. 配置及导出物不含 Token，测试覆盖泄漏防线。
3. Account、Runtime Profile、继承路由、密钥后端和两个 Adapter 可独立测试。
4. `setup/add/use/rotate/status/test/verify/rollback/doctor/sync` 形成完整日常闭环；完整验证必须证明真实响应的 sentinel，不能以 HTTP 2xx 或进程退出码替代。
5. 团队清单可导入且不能携带密钥。
6. 安装、升级、卸载不覆盖非 AIGW-owned 用户配置；native 安装通过 native 包更新，portable 安装才允许原子替换自身。
7. 本机旧原型迁移后，`aigw doctor`、`aigw test` 和客户端边界验证通过；`doctor` 能识别 Codex 顶层模型、provider 选择或 provider 块的漂移。
