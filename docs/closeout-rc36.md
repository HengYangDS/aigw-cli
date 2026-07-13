# RC.36 本地收口记录

**状态：** 本地 RC 候选已收口；远端发布、签名与 GA 仍未发生。
**范围：** 本记录只覆盖 `0.1.0-rc.36` 的本地源码、工件、本机安装与客户端连通性证据。

## 结论

RC.36 修补了一个发布证据缺口：此前 Linux 运行验收只覆盖 `amd64`，且容器内会为了
安装工具访问网络。现在验收覆盖 `amd64` 与 `arm64` 的 `.deb` 和 `.rpm` 四条路径；
候选镜像准备完成后，每条工件安装和执行均在 `--network none`、`--pull never` 的容器中
进行。

这使本地结论精确到：**同一 RC.36 工件矩阵完整，四个 Linux 原生包在对应架构的
Debian/RPM 系兼容容器中可离线安装并执行。** 它不等同于 Debian/Fedora 受管 runner
原生证据，更不等同于 GitLab Release 已发布或 GA 已签名。

## 已验证的事实

| 结论 | 验证方式 | 结果 |
|---|---|---|
| 源码质量门 | `go test -race ./...`、`go vet ./...`、脚本语法和发布门禁 | 通过 |
| RC 工件完整 | `AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh 0.1.0-rc.36 dist` | 15 工件通过 |
| 工件布局与校验和 | `check-release-artifacts.sh`、`test-release-package-layout.sh` | 通过 |
| Linux Debian 安装 | `linux/amd64`、`linux/arm64` 各安装 `.deb` 并运行 `/usr/bin/aigw --version` | 通过 |
| Linux RPM 安装 | `linux/amd64`、`linux/arm64` 各安装 `.rpm` 并运行 `/usr/bin/aigw --version` | 通过 |
| 本机程序升级边界 | 用校验和匹配的 macOS arm64 便携包升级 `~/.local/bin/aigw` | 通过；保留一个程序回退副本 |
| 配置未被升级触碰 | 比较 AIGW 配置、Codex 配置、`.zshrc`、`.zshenv` 的 SHA-256 | 未变化 |
| 本机服务可用性 | `aigw check`、`aigw doctor`、`aigw test --for claude`、`aigw test --for codex` | 通过；两条端点均为 HTTP 200 |

本地候选目录为 `/private/tmp/aigw-rc36-linux-runtime-acceptance`。其存在只构成当前
工作站的本地证据；分发前必须重新基于目标提交和目标版本构建并验证。

## 本轮变更

- Linux 验收改为 Debian 与 RPM 系兼容镜像，按 `amd64`、`arm64` 分别验证 `.deb` 与
  `.rpm`。
- 候选镜像准备与工件安装分离：镜像可按需拉取；实际工件执行强制 `--network none` 与
  `--pull never`。
- 共享临时目录只承载四个待测 Linux 包；每轮退出后清理。
- 验收脚本的自测检查架构、包格式、离线网络、禁止运行阶段拉取及临时目录清理。
- 发布证据契约与 README 同步为真实验收范围，避免把结构检查或旧日志误称为运行证据。

## 经验教训

1. **工件存在不是可用性证据。** 交叉编译、压缩包解包和元数据检查只能证明结构；安装
   程序、架构字段和运行时动态装载必须在目标包管理器路径上实际执行。
2. **架构是验收维度，不是构建参数。** 发布 `amd64` 与 `arm64`，就必须分别取得两者的
   运行证据；以一个架构替代另一个架构会留下不可见缺口。
3. **将镜像准备与工件执行分开。** 容器镜像拉取可以是外部前置条件；一旦进入候选工件
   验收，容器网络必须关闭，避免包安装偷偷依赖网络或仓库状态。
4. **升级验收先证明边界，再证明可用。** 先比较受保护配置的校验和、确认只替换 AIGW
   程序并保留回退副本；再运行 `check`、`doctor` 和两条客户端测试。这样故障定位不会
   把程序升级、配置漂移与网关问题混在一起。
5. **人类诊断与机器诊断应分层。** 人类命令不应泄露 TOML 路径或解析细节；详细原始
   诊断仅留给显式机器接口。恢复动作应优先指向最小安全命令，而不是惯性建议广泛修复。
6. **Housekeeping 不等于清空历史。** 保留当前候选工件、校验和、提交和失败/成功证据；
   只删除当前轮可再生的 scratch。未知归属的历史工作树、旧工件和会话不能因名称相似
   而被删除。

## 明确未完成项

- GitLab 不可达时未推送、未创建 tag、未上传工件，也未验证远端 Release。
- 未取得受管 Debian/Fedora 原生 runner 证据；本地容器兼容性验收不能替代它。
- 未建立 macOS Developer ID 签名/公证和 Windows Authenticode/时间戳的 GA 证据；因此
  不得创建或声称 GA。
- `aigw verify` 会消耗额度；本轮只执行无模型调用的 `test`，未将其替代为真实模型
  验证。

## 恢复远端后的最小路径

1. 重新确认 GitLab、远端分支、CI runner 与签名条件；不要使用本记录代替即时状态。
2. 从干净目标提交以最终 RC 版本重新构建、验证并发布；不要上传本机临时目录中的旧工件。
3. 校验 GitLab Release 的实际资产、`checksums.txt` 和 SBOM，再从远端工件做干净安装。
4. 只有受保护 CI 已对同一发布资产给出 macOS 与 Windows 签名证据后，才评估 GA。
