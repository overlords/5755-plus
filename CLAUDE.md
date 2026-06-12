# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 仓库性质

本仓库是 5755 SDK v2 的**文档 + SDK + 平台服务端同仓**项目:`docs/` 下是 8 份权威需求/规格/验收文档与 UI 资产;Android SDK(Gradle 工程,从零重写)与平台服务端(从零重构,一期只含 SDK 网关面)都在本仓库内开发,以 04 契约为两端共同口径(尚未脚手架时仓库暂为纯文档状态)。参考实现:旧 SDK 在 `~/Developer/m5755`(纯 Java,带 B2-B5 缺口),旧平台原型在 `~/Developer/U10`(Node/TS,现行 dev 部署的承载者);两者都**只作阅读参考,不搬运**(见 `docs/adr/`)。新实现以本文档集为唯一口径。

所有文档使用简体中文撰写;术语、字段名、API 名保留原文(如 `account`、`Order`、`devCode`)。

## 文档体系与阅读顺序

文档间存在严格的权威分层,修改时必须维持一致性:

- `docs/01-product-scope.md` — 产品范围、一期 14 项能力白名单、公开 API 白名单(`com.m5755.operate.api.*`)、依赖/能力/Manifest 排除项。**任何能力进出产物必须先修订本文**,其余文档随之对齐。
- `docs/02-terminology.md` — 114 条术语 + 6 组易混淆对照,是**全项目文档与代码命名的唯一口径**;其他文档或代码与它冲突时,以它为准修正(除非有意修订术语本身)。
- `docs/03-entry-account-flow.md` — 主流程状态顺序、各节点阻断/回退规则(含"是否触发账号变化"判定表)、游戏小号体系、登出语义。
- `docs/04-platform-gateway-api.md` — SDK 内部网关与平台服务端的 HTTP JSON 契约 v2:9 条资源式路径(HTTP 方法区分语义)、`ApiResult` + 机器可读 `reason` 枚举(失效分流的唯一依据)、HMAC-SHA256 入站签名、dev 控制面(`/internal/dev-control/*`,异常注入)、充值回调(服务端)、环境矩阵;**这是 SDK 与平台服务端两端的共同验收口径**。
- `docs/05-payment-role-report.md` — 角色上报、支付流程与 `Order` 语义、客户端支付回调 vs 充值回调责任矩阵。
- `docs/06-user-center.md` — 悬浮球入口、H5 容器安全边界、JS Bridge 最小契约(仅 `getAccountContext` / `postAccountAction` 两个方法)。
- `docs/07-ui-spec.md` — 12 个界面的布局/文案/颜色/尺寸规格与通用规范。**视觉上游**是 claude.ai 设计系统项目「5755 SDK Design System」(经 DesignSync 工具读取,仓库不存快照);设计产出进实现必须先修订 07,07 仍是实现唯一口径;Android 不内嵌字体(07 §1.11)。
- `docs/08-acceptance.md` — 验收三面(样例演示 + 诊断快照 + 上线阻断回归)、12 个模拟器验收场景、旧实现已知缺口 B2-B5。

资产:`docs/assets/`(业务流程总图、`acceptance/` 验收截图、`audit-2026-06-12/` 审计证据截图)。

## 仓库目录布局

```
docs/        # 8 份权威文档 + adr/ + agents/(共享)
android/     # Android SDK Gradle 工程根:sdk-core / sdk-ui / sdk / sample(AGP 8.13.1,minSdk 21)
server/      # 平台服务端 Go 工程根(module m5755/server)
scripts/     # 跨端脚本:部署 sdk-dev、发布门禁探测等
.scratch/    # 本地工作区,不入库
```

`server/` 内部:`cmd/server/` 入口 + `internal/{api,signature,devcontrol,domain,store}`,贴 04 契约三个面切包;`devcontrol` 用 Go build tag 实现"生产构建不注册路由"(04 三重生产防护①)。顶层不叫 `sdk/`(避免与 Gradle 模块 `:sdk` 形成 `sdk/sdk/` 路径),服务端目录不叫 `gateway`(术语表禁用)。

## 贯穿全部文档的核心不变量

编辑任何文档时,以下口径不得被破坏:

- **`account` 固定指当前游戏小号 ID**,绝不是 5755 账户(平台主账户)ID;主账户用 `platformAccountId` / `platformToken`,小号登录令牌用 `token`。SDK 不向游戏暴露 5755 账户 ID。
- **游戏小号一律由服务端返回,不可凭空造演示小号**;列表为空属平台侧异常,阻断登录并输出诊断。
- **客户端支付回调 ≠ 充值回调**:前者仅 UI 提示,后者(平台→游戏服务端)才是物品发放的唯一依据;AAR 不内置服务端验签/发放逻辑。
- **账号变化事件收敛边界**:退出登录、切换小号、踢号、账户失效、小号失效触发账号变化;维护门禁、协议拒绝、防沉迷进入/支付门禁阻断**不**触发账号变化。
- **自动登录失败必须回退展示 5755 账户登录窗口**,不得以失败码结束流程;本地登录态不能替代服务端有效检查放行。
- **失效分流只依赖 `reason` 枚举,不解析 message 文本**:`platform_account_invalid` → 回登录窗口,`subaccount_invalid` → 进小号选择页,两者不得混用;公开 `OperateCode` 7 码不扩。
- 环境、渠道、签名配置只能由构建/发布配置固定,**不允许透传参数或运行时切换**;渠道异常一律回退 `default` 且不阻断。
- 旧实现已知缺口(08 文档 B2-B5:自动登录跳过服务端校验、空列表伪造演示小号、设备验证空实现、结果弹窗写死样例)**不得**带入新实现或新文档表述。

## 项目环境决定(写死的口径)

- v2 开发与联调一律对接新平台服务端 dev 部署 `sdk-dev.xingninghuyu.com`(`artifactType=integration`、`platformEnv=dev`,契约前缀 `/api/sdk/v2/*`);生产为 `sdk.xingninghuyu.com`。`dev.xingninghuyu.com`/`api.xingninghuyu.com` 与 `/api/sdk/v1/*` 属旧平台原型(U10,在 `~/Developer/U10`),服务现有外部使用方,不可扰动。
- 登录凭证自给自足:平台服务端短信 mock provider 返回 `devCode`(部署权在本项目手里),无需外部申请。
- 诊断日志 tag 为 `M5755Sdk`;样例包名 `com.m5755.sdk.ui.sample`;本地存储文件 `shared_prefs/m5755_operate_min.xml`。

## 语言规范
- 所有对话和文档使用中文
- 文档使用markdown格式

## Agent skills

### Issue tracker

Issues 记录在本仓库的 GitHub Issues(使用 `gh` CLI)。见 `docs/agents/issue-tracker.md`。

### Triage labels

五个 triage 角色使用默认标签名(needs-triage / needs-info / ready-for-agent / ready-for-human / wontfix)。见 `docs/agents/triage-labels.md`。

### Domain docs

单上下文布局;领域术语的权威来源是 `docs/02-terminology.md`。见 `docs/agents/domain.md`。
