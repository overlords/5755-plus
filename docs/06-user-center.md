# 06 用户中心(H5 容器与 JS Bridge)

本文档定义 SDK 用户中心的入口、H5 容器边界、最小 JS Bridge 契约和账号动作语义。用户中心属于玩家进入游戏后的操作行为;其 Bridge 是 SDK UI 内部桥接,不是接入方公开 API。

字段口径:`accountId` 指当前**游戏小号** ID;"5755账户"指平台主账户。用户中心只围绕游戏小号上下文工作,不维护、不切换 5755 账户。

## 1. 入口:悬浮球

- 用户中心通过悬浮球扩展打开,属于游戏进入后的玩家操作行为。
- v2(最小化 AAR)悬浮球**只保留用户中心一个入口**;可选平台工具(客服、IM、活动、优惠券、平台 App 跳转等)不进入当前包。
- 小号自动进入提示不属于用户中心,也不是游戏小号选择页;小号选择页右侧的"切换 5755 账户"入口也不是用户中心,三个 UI 场景不得混淆。

## 2. H5 容器边界

用户中心 H5 由 Android 系统 WebView 和必要 JS Bridge 承载,容器保持最小:

| 容器项 | 约束 |
| --- | --- |
| WebView | Android 系统 WebView,SDK 自有轻量页面容器(抽屉式),不依赖完整 SDK 页面体系。 |
| JavaScript | 开启(`setJavaScriptEnabled(true)`);禁止 JS 自动开窗(`setJavaScriptCanOpenWindowsAutomatically(false)`)。 |
| 文件访问 | 关闭:`setAllowFileAccess(false)`、`setAllowFileAccessFromFileURLs(false)`、`setAllowUniversalAccessFromFileURLs(false)`。 |
| 排除能力 | 文件上传、媒体选择、APK 下载/安装、外部 App 跳转、客服 H5 与通用 H5 能力一律不开放。 |

H5 内容边界:

- 用户中心 H5 只能通过最小 JS Bridge 读取当前游戏小号上下文,**不维护另一套当前账号**。
- H5 只把账号动作回传 SDK,实际状态变更全部由 SDK 状态机执行。

## 3. JS Bridge 最小契约

SDK 通过 `addJavascriptInterface` 向页面注入名为 `UserCenter` 的桥对象(`window.UserCenter`),仅暴露两个方法:

| Bridge 方法 | 参数 | 返回 | 说明 |
| --- | --- | --- | --- |
| `getAccountContext()` | 无 | JSON 字符串 | 返回当前游戏小号上下文,当前仅含一个字段:`{"accountId":"<当前游戏小号 ID>"}`。 |
| `postAccountAction(action)` | `String action` | 无 | H5 向 SDK 回传账号动作。仅接受 `logout`、`switch_account`、`session_invalid` 三个取值;其他任何取值在进入 SDK 回调前归一为 `unknown`。回调在 UI 线程派发。 |

契约约束:

- Bridge 暴露面固定为上述两个方法,不得扩大;不提供任何文件、媒体、下载、跳转或通用能力方法。
- `getAccountContext` 的返回是最小集合,只有 `accountId`;新增字段需重新评审。
- 非法动作值不抛错、不透传,统一归一为 `unknown`,由 SDK 侧忽略或仅做诊断记录。

## 4. 账号动作语义

| 动作 | 语义 |
| --- | --- |
| `switch_account` | **切换游戏小号**(不是切换 5755 账户)。SDK 进入游戏小号选择页,由玩家明确选择目标小号后才完成切换;不展示小号自动进入提示,不允许静默自动切换到下一个小号;玩家取消切换时保持当前游戏小号,当前游戏会话不被更改。没有当前游戏小号时,不得为了切换而隐式触发初始化、登录或退出登录。 |
| `logout` | **退出登录**。SDK 清理 5755 登录态和当前游戏小号,触发账号变化通知,返回 5755 账户登录窗口;不重新展示协议告知。 |
| `session_invalid` | 登录态失效类动作,由 H5 回传 SDK 处理账号失效。 |
| `unknown` | 非法动作的归一值,不触发任何账号状态变更。 |

红线:

- **用户中心不提供切换 5755 账户**。切换 5755 账户只能通过退出登录后重新登录完成。
- 切换游戏小号与退出登录必须区分:只有 `logout` 才返回 5755 账户登录窗口。

## 5. 阶段说明

### 5.1 当前(v2):SDK 本地最小容器

- 用户中心内容为 SDK 内置本地 HTML,通过 `loadDataWithBaseURL(null, ...)` 加载;**baseURL 为 `null`(不可信 origin),不伪造任何平台域名**(如 `https://sdk.5755.local` 之类不存在的域名)。
- 本地容器仅提供:当前游戏小号上下文展示、切换小号、退出登录;页面通过 `UserCenter.getAccountContext()` 同步小号 ID,通过 `UserCenter.postAccountAction(...)` 回传动作。
- 更多账号服务由平台用户中心 H5 提供,属未来范围。

### 5.2 未来:接入平台真实 H5

- 未来接入平台真实用户中心 H5 URL,替换本地容器内容;Bridge 契约(第 3 节)保持不变。
- 接入时必须完成安全评审,至少覆盖:
  - **Bridge 暴露面**:确认仍仅暴露 `getAccountContext` / `postAccountAction` 两个方法,无新增能力泄漏;
  - **origin 校验**:加载真实 URL 后,桥对象暴露给的页面来源必须受控,需评审对加载域名/来源的校验策略,防止任意页面调用 Bridge;
  - 容器禁用项(文件/媒体/APK/外跳/通用 H5)保持关闭。
