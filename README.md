# AIGW CLI

跨平台、无后台服务的团队 AI API 配置工具。

`aigw` 统一管理第三方 AI API Profile、系统密钥、Claude/Codex 路由与客户端适配；团队共享端点清单，每位成员的 Token 只保存在本机安全密钥库。

> 当前分支正在构建首个产品版本。完整契约见 [产品设计](docs/design/2026-07-10-aigw-cli-product-design.md)。

## 核心约束

- 一个 Profile 对应一个服务商实例和一个 Token。
- 默认路由可被 Claude 或 Codex 单独覆盖，未覆盖时自动继承。
- Token 不进入 TOML、JSON、命令行参数、日志、文档或备份。
- Claude 与 Codex 各自通过独立 Adapter 投影，互不污染配置目录。
- 工具按命令运行后退出，不安装 daemon、watchdog 或本地代理。

## 目标平台

- macOS arm64 / amd64
- Linux arm64 / amd64
- Windows arm64 / amd64

## License

Internal use. See repository visibility and group policy.
