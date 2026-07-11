# Adapter 准入

## 边界

AIGW 区分两件事：

1. **Provider Account 准入**：一个上游服务商、其协议端点和一把独立 Token。
2. **Client Adapter 准入**：一个本地客户端如何安全读取已解析的 Profile、如何保存或
   不保存配置、如何验证及回退。

Provider 支持某协议不等于 AIGW 已支持一个新客户端。只有当前已证明的 Claude 与
Codex Adapter 可以被启用；其他客户端不能通过手工填写模型名、复用 Claude/Codex
目录或把 OpenAI-compatible 宣称为 Codex Responses 来绕过准入。

运行时将已准入客户端收敛在一个静态注册表中。状态、诊断、Profile/Route 校验和
Adapter 列表均从该表读取；新增候选模型不会改变这个列表。只有新 Adapter 完整通过
本页的证据记录并在代码中注册后，它才会出现在正常命令和团队清单的可选范围。

## 当前已准入边界

| 客户端 | 配置/认证边界 | 可使用的 Provider Account |
|---|---|---|
| Claude Code | AIGW 专属 shim；仅启动目标进程时映射 Anthropic 环境变量 | 已验证 Anthropic 兼容 Account |
| Codex | AIGW-owned `config.toml` 投影；官方 `login --with-api-key` | 已验证 OpenAI Responses Account |

每个 Account 仍然独立持有一把系统 Token。切换同一 Account 下的模型不会复制 Token；
切换 Account 也不会把 Token 写入客户端配置。

## 候选能力与官方协议证据

| 候选 | 正确定位 | 已核实的官方接口 | 目前状态 |
|---|---|---|---|
| GLM Coding Plan | **独立 Provider Account**，不是虚构的 `glm` client | Z.AI 为 Claude Code 提供 Anthropic 端点，为 OpenCode 提供 coding PaaS 端点 | Claude 协议路径可作为独立 Account 评估；OpenCode Adapter 未准入 |
| Gemini CLI | 独立 Client Adapter | `GEMINI_API_KEY`；系统/用户/项目/环境/命令行的配置优先级 | 未准入 |
| Qwen Code | 独立 Client Adapter | `modelProviders` 的 `envKey` 读取环境变量；密钥不应持久写入 settings | 未准入 |
| OpenCode | 独立 Client Adapter | `OPENCODE_CONFIG_DIR`；配置可使用 `{env:NAME}` 注入 | 未准入 |
| Perplexity | 研究能力 Provider，非 Codex 默认 | Agent API 提供 Responses-compatible 路径；Sonar Deep Research 用于研究 | 未准入；不得因兼容声明成为 Codex 路由 |
| Grok | 交叉核验备用 Provider | 需在实际选择的客户端协议下单独验证 | 不进入日常模板 |

来源（2026-07-11 核验）：

- [Z.AI Developer FAQ](https://docs.z.ai/devpack/faq)
- [Gemini CLI configuration](https://github.com/google-gemini/gemini-cli/blob/HEAD/docs/reference/configuration.md)
- [Gemini CLI authentication](https://github.com/google-gemini/gemini-cli/blob/HEAD/docs/get-started/authentication.md)
- [Qwen Code model providers](https://qwenlm.github.io/qwen-code-docs/en/users/configuration/model-providers/)
- [OpenCode configuration](https://opencode.ai/docs/config/)
- [Perplexity OpenAI compatibility](https://docs.perplexity.ai/docs/agent-api/openai-compatibility)
- [Perplexity Sonar Deep Research](https://docs.perplexity.ai/docs/sonar/models/sonar-deep-research)

这些资料只证明候选的协议入口；它们不构成 AIGW 的运行时适配、工具调用兼容或模型
效果证据。

## 必须提交的准入记录

每个新 Adapter 必须在合并前提交以下完整记录：

1. **版本与可执行文件**：客户端名称、精确版本、macOS/Linux/Windows 安装来源。
2. **专属配置边界**：实际读写文件、状态文件及卸载范围；不得写入 Claude/Codex
   目录或共享 shell Token。
3. **协议合同**：端点类型、认证头/环境键、模型选择、流式、工具调用、必要的图像与
   长上下文行为。
4. **密钥证明**：Token 仅来自 `AIGW_TOKEN/<account>`；配置、日志、命令行、团队清单
   和备份均无密钥。
5. **回退证明**：停用后恢复的文件与状态 hash；用户手工漂移时必须停止而非覆盖。
6. **最小真实验证**：在用户明确同意消耗额度后，记录 Profile、时间、响应 sentinel
   和非敏感诊断结果。
7. **可用性判断**：效果、稳定性、成本、区域可达性、许可证和维护负担；每个能力席位
   仅保留一个首选项。

在以上记录齐备前，CLI 应保持“未准入”状态：不出现可启用 Adapter、不进入团队模板、
不创建可路由 Profile，也不尝试自动 fallback。

## GLM 的正确前置准备

未来如接入 Z.AI GLM Coding Plan，应新建一个例如 `zai-coding-plan` 的 Account，并只
在用户交互录入该 Account 的独立 Token。若选择其 Claude Code 协议端点，它由现有
Claude Adapter 在进程边界映射，不写 `~/.claude/settings.json`，更不触碰 `~/.codex`。

OpenCode 路径则必须等其 Adapter 完成上述准入记录后再实现。两条路径的 Account、
Token、验证与回退证据不可互相假定。

## 永久不作为默认

Perplexity 只承担研究与引文；Grok 只作备用交叉核验。DeepSeek、Kimi、MiniMax 不在
推荐集、团队模板或默认路由中。模型目录的存在不改变这些边界。
