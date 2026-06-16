# v2 不设 onGameStart 生命周期钩子,init 为唯一启动入口

状态:已接受(2026-06-16)

## 背景

旧 SDK(`~/Developer/m5755`)的 `onGameStart(Context)` 有真实职责:`SdkRuntime.onGameStart` 捕获 **application Context**、据此惰性建 `store`(`LightStore`)、记一条生命周期诊断,并被 `rememberActivity()` / `setOptions()` 多个引导路径调用。它存在的意义是「在 `init` 前先把 app 级 Context 与存储立起来」。

v2 重写时把这套模型**整个设计掉了**:`init(Activity, Options, Listener)` 直接吃 Activity、`storage` 在 `init` 内从 Activity 建、`login` / `recharge` / `changeUser` / `shouldQuitGame` / `destroy` 全程 Activity 穿线、SDK **不持 app Context**。`onGameStart` 却被保留为公开门面方法(对齐旧签名),实现退化为**纯空操作**(只打一行日志、连 `Context` 参数都忽略),并仍被列进 `01 §3` 公开白名单、`03` 主流程、`integration-guide §3`。

一次 grill(2026-06-16)查清:`onGameStart` 在 v2 **零职责、零调用价值**;`03 §2.1` 把它与 `init` 并列却把全部实事(配置拉取/渠道诊断/接入自检/维护门禁)归 `init`;术语表(`02`)无其词条。它正面顶撞 `01 §3` 两条原则——「只保留最小上线链路需要的方法」与「**范围外旧入口不做空实现兼容**」。

## 决定

**v2 移除 `onGameStart` 公开方法;`init` 成为唯一启动入口。**

同步改动:
- `Operate` 删 `onGameStart` 方法 + 现已无用的 `import android.content.Context;`。
- `01 §3` 白名单删 `onGameStart`(行 `SDK 版本与启动 | getVersion、onGameStart` → `SDK 版本 | getVersion`)。
- `03`:`§1` 主流程删 `onGameStart` 节点(`init` 成为入口),`§2.1` 标题与正文以 `init` 为前置。
- `integration-guide §3`:调用顺序 `init → login → …`;`§3.1` 代码删 `Operate.onGameStart(activity);`。
- 样例按钮 `进入游戏(onGameStart + init)` → `进入游戏(init)`,连带 6 处仪器化 tap 文案 + `scripts/e2e-channel.sh` 同步。

## 为什么

- **空操作公开方法 = 出货公开面噪声**,违反最小化 AAR 与 `01 §3`「最小上线链路 + 不做空实现兼容」。与同期移除的死公开 API `setPlatformConfigOverride`(见提交历史)同理——纯净公开面卫生。
- **原本的存在理由已被设计掉**:app Context + store 引导在 v2 的 Activity 穿线模型下不存在,`onGameStart` 无替代职责。
- **贴「范围外旧入口不做空实现兼容」**:从旧 SDK 迁来者调旧入口应在编译期暴露,而非吃一个静默空壳。

## 考虑过的其他选项

- **保留作为预留钩子**(为未来防沉迷在线时长 / 行为上报 NPPA `loginout` 留一个「会话开始」锚点)——**否决**:该能力属 v2 范围外·future;YAGNI,真要时随能力一并加(带真实行为 + 重走 `01 §5` 评审),而非现在留空壳。空壳预留还会被后人误读为「有功能的前置」(`03 §2.1` 当前正是这样装相)。
- **现在就给它真实职责**(传 application Context / 记会话开始时间)——**否决**:v2 全程 Activity 穿线、不需要 app Context,无真实需求。
- **保留现状不动**——**否决**:见「为什么」。

## 后果

- 公开 API 白名单(`01 §3`)收窄一项;`init` 为唯一入口,调用顺序 `init → login → …`。
- **这是公开契约变更**。pre-GA、无外部接入方(首款游戏适配中),成本低;GA 后若要再引入「会话开始」语义,应作为**带真实行为的新能力**走 `01 §5` 评审,而非恢复空壳。
- 接入方迁移:从旧 SDK 迁来者不再调 `onGameStart`(编译期暴露,符合「不做空实现兼容」),改以 `init` 为起点。
- 验证(JDK21)::sdk 编译 + `verifyPublicAarPurity`(五维)+ `:sdk:test` 全绿;样例 / 仪器化 / e2e 文案同步。
