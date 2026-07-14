# AIGW CLI 产品设计

> **Historical design record.** Superseded by the canonical documentation root and current implementation. It remains only for provenance.


**状态：** 已确认  
**产品名：** AIGW  
**命令：** `aigw`  
**仓库：** `dig/misc/agentic-third-party-api/aigw-cli`

## 1. 定位

AIGW 是**本机优先、团队可分发**的跨平台第三方 AI API Account、模型 Profile、密钥与客户端路由工具。它是一个按需执行的本地 CLI，不承载 API 流量，不运行后台服务，也不是中央密钥分发系统。

仓库名使用 `aigw-cli`，明确它是管理客户端而非真正的数据面网关。上游服务商名称仅是可配置的 Account 标签，不是产品身份。

### 1.1 优先级

1. **本机独立可用：** 用户安装一个原生二进制后，即可用系统密钥存储配置任意多个服务 Account、其下任意多个模型 Profile，以及 Claude、Codex 和未来客户端的显式 Route；每个 Account 可直连其 HTTPS 入口，不依赖内网、团队服务、本地端口或常驻进程。
2. **团队低摩擦复用：** 团队只分发经过审查的无密钥 Account/Profile 清单和签名 Release；每位成员保有自己的 Token、个人路由和客户端状态。
3. **集中数据面按需引入：** 仅当集中审计、预算、协议转换或组织出口成为明确需求时，另行选择并部署独立 Gateway。它不是 AIGW 的功能、依赖或默认路径；AIGW 只使用其 HTTPS URL，绝不管理其进程、端口、上游密钥或故障转移策略。

因此，LiteLLM、Bifrost 和同类数据面软件不进入 AIGW 安装包、本机默认流程或日常 UX。它们未来若被组织采用，必须在独立项目中以真实场景重新评测，且不改变本机多服务、多模型、本地 Route 的可用性。

## 2. 领域模型

### Account

一个 Account 表示一个服务商实例、它支持的协议端点及一个 Token。相同服务商的不同 Token 必须建立不同 Account，例如 `primary-gateway` 与 `backup-gateway`。

### Runtime Profile

一个 Runtime Profile 表示用户日常切换的模型运行配置，引用一个 Account，并可限定客户端与模型名，例如 `gpt-5.6-terra`、`claude-fable-5`、`claude-sonnet-5`、`claude-opus-4-8-thinking`。Profile 可选 `purpose` 作为一行人类用途提示；它不参与路由或密钥管理。

`claude-fable-5` 是 Claude 的推荐基线；其他 Claude Profile 必须显式选择。模型名对 AIGW 是透明字符串，由上游网关解释。

一个 Account 可以被多个 Runtime Profile 引用；Profile 不拥有 URL 或 Token。于是一把 Account Token 可以安全地支撑 GPT、Claude、Embedding 等多个模型选择；另一个服务商则增加另一个 Account 及其一把 Token。Profile 是本机多服务、多模型能力的基本单位，不是服务端 Gateway 的替代物。

### Provider Diagnostics

通用核心不内置任何服务商、URL、Token 槽位或模型清单。`check` 基于 Account 端点提供通用诊断；精确余额、Token 状态等私有管理 API 仅能作为显式 Provider Diagnostics 集成。清单可声明其类型，而当前二进制未包含该驱动时，配置与路由仍然有效，`balance` 只给出清晰的不可用说明。这样新增服务商不改变核心模型，也不会把当前服务商误写成产品身份。

### Endpoint

Account 可提供 `openai-responses` 和 `anthropic` 两类端点，它们共享该 Account 的唯一 Token。端点是协议能力，不是客户端配置。Runtime Profile 只选择模型和客户端作用面。

### Route

Route 将客户端使用面映射到 Runtime Profile。解析顺序为：单次命令的 `--profile`、客户端覆盖路由、默认路由。未设置客户端覆盖时自动继承默认路由，不保存重复副本。

Route 是显式的预请求选择：Claude shim 在每次 Claude 启动边界解析，Codex 在 AIGW 管理的配置投影边界解析。AIGW 没有本地数据面，因此不会、也不应在一个已发出的模型请求中切换服务商或模型；自动 fallback 默认禁用。

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

Claude 通过独立 shim 启动，Token 仅在目标进程边界映射为 `ANTHROPIC_AUTH_TOKEN`，端点映射为 `ANTHROPIC_BASE_URL`。映射不持久化到 Claude 设置文件或 shell 启动文件。shim 位于 AIGW 专属数据目录；为保持原生 `claude` UX，AIGW 只在当前用户 shell profile 写入一个有标记、无密钥、可逆的 PATH 块，不再使用共享 `~/.local/bin`。

Codex CLI 优先通过独立执行边界传递凭据。对无法经 shim 启动的 Codex Desktop，AIGW 只在首次启用、Account 切换、Token 轮换或显式 `aigw adapter auth codex` 时通过 Codex 官方 `login --with-api-key` 边界刷新认证；模型切换和 `aigw sync` 不触发登录。AIGW 只维护带 AIGW 标记的顶层 `model`、`model_provider` 与 provider 配置，不得直接写入明文认证文件，也不得启动、停止或重启桌面客户端。

## 5. 用户体验

首次运行 `aigw` 或 `aigw setup` 由通用向导完成 Account、首个模型 Profile、客户端、端点和隐藏 Token 的输入；也可先导入团队清单。它不预置任何供应商。配置变更自动同步；同一 Account 内的模型切换不重复绑定认证；`aigw sync` 仅用于恢复漂移，并且不改变认证或客户端进程状态。

日常命令保持在一个屏幕内：

```text
aigw
aigw add <account>
aigw account edit <account>
aigw profile add <profile> --account <account> --for claude|codex --model <model> [--purpose <hint>]
aigw use <profile> [--for claude|codex] [--all]
aigw rotate <account>
aigw catalog [--all|--json]
aigw status [--json]
aigw test [--for claude|codex]
aigw verify --for claude|codex|all
aigw rollback [--last-change]
aigw doctor [--json]
aigw sync
```

`test` 只检查端点和认证；`verify` 是显式付费动作，发送一次最小真实模型请求并要求精确回显 `AIGW_OK`。`verify --for all` 仅在 Claude shim、Codex 投影和两条真实协议链路都通过后保存不含密钥的验证检查点。`rollback` 优先恢复该检查点，`--last-change` 仅恢复上一份配置备份；两者都不会控制客户端生命周期。

高级能力收进 `profile`、`route`、`adapter`、`config` 和 `completion` 命名空间。命令集只实现当前领域模型。错误必须同时给出原因、影响和一条可执行修复命令。

## 6. 配置与团队复用

本机配置使用 TOML，仅包含 Account 元数据、端点、Runtime Profile、模型名、路由、Adapter 状态与可执行文件路径。团队清单可安全提交到仓库，包含 Account、Profile、URL、协议、模型名和推荐默认路由，但不包含 Token、个人覆盖路由、客户端登录状态或本机路径。

配置位置遵守平台约定：

- macOS：`~/Library/Application Support/aigw/config.toml`
- Linux：`${XDG_CONFIG_HOME:-~/.config}/aigw/config.toml`
- Windows：`%APPDATA%\aigw\config.toml`

所有配置写入使用锁、校验、备份、原子替换和失败回滚。Adapter 只恢复自己曾修改且未被用户再次更改的字段；冲突时停止而不是覆盖用户内容。

## 7. 技术与交付

采用 Go 构建单文件二进制，无 Python/Node 运行时要求。管理命令执行后退出；不安装 daemon、watchdog、launchd、systemd 或计划任务。Claude/Codex shim 只负责进程边界转发。Claude shim 位于用户级 AIGW shim 目录，不能放入 `~/.codex`，也不能写入包管理器拥有的系统目录。

GitLab Release 为 macOS、Linux、Windows 的 amd64/arm64 生成 portable 压缩包、原生安装包、SHA-256 校验和与 SBOM。原生安装包包括 macOS Universal `.pkg`、Linux amd64/arm64 `.deb` 与 `.rpm`、Windows amd64/arm64 `.msi`。portable 包内的 `install.sh` 和 `install.ps1` 只复制同包二进制，不实现任何网络下载、Token 处理或 Release 选择；已安装 CLI 的网络升级唯一收口于 `aigw update`。包安装与卸载只管理包拥有的文件，绝不遍历用户目录、创建或删除任何用户 shim；shim 始终由该用户运行的 AIGW Adapter 命令管理。

## 8. 现有资产边界

传输数据面网关是独立项目，不进入 AIGW 核心，也不由 AIGW 管理。AIGW 默认使用上游 HTTPS 直连且不绑定本地端口。若组织以独立项目部署网关，AIGW 只把它视为 HTTPS Account 端点。

## 9. 验收标准

1. 六个目标 OS/架构均可交叉编译。
2. 配置及导出物不含 Token，测试覆盖泄漏防线。
3. Account、Runtime Profile、继承路由、密钥后端和两个 Adapter 可独立测试。
4. `setup/add/use/rotate/status/test/verify/rollback/doctor/sync` 形成完整日常闭环；完整验证必须证明真实响应的 sentinel，不能以 HTTP 2xx 或进程退出码替代。
5. 团队清单可导入且不能携带密钥。
6. 安装、升级、卸载不覆盖非 AIGW-owned 用户配置；native 安装通过 native 包更新，portable 安装才允许原子替换自身。
7. `aigw doctor`、`aigw test` 和客户端边界验证通过；`doctor` 能识别 Codex 顶层模型、provider 选择或 provider 块的漂移。
