# 06 用户中心(H5 容器与 JS Bridge)

本文档定义 SDK 用户中心的入口、H5 容器边界、最小 JS Bridge 契约和账号动作语义。用户中心属于玩家进入游戏后的操作行为;其 Bridge 是 SDK UI 内部桥接,不是接入方公开 API。

字段口径:`accountId` 指当前**游戏小号** ID;"5755账户"指平台主账户。用户中心内容**以主账户为核心**(平台 H5 凭 `platformToken` 自取),不向游戏暴露主账户、也不维护或切换 5755 账户。

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
| JS dialog | `WebChromeClient` 仅处理 `onJsAlert`/`onJsConfirm`(`alert`/`confirm` 弹原生 `AlertDialog`,支撑远程页二次确认如退出登录;无此则 `confirm()` 默认返回 false、确认静默失效)。**不处理 `onShowFileChooser`**——文件选择与下方「文件上传」排除一致。 |
| 排除能力 | 文件上传、媒体选择、APK 下载/安装、外部 App 跳转、客服 H5 与通用 H5 能力一律不开放。 |

H5 内容边界:

- 用户中心 H5 **不经 bridge 读取账户上下文**(`getAccountContext` 已移除);主账户内容由平台页凭 `platformToken` 自取,**不维护另一套当前账号**。
- H5 只把账号动作回传 SDK,实际状态变更全部由 SDK 状态机执行。

## 3. JS Bridge 最小契约

用户中心 H5 由**平台远程页**承载(见 §5),**主账户内容由平台 H5 凭 SDK 传入的 `platformToken` 自取**;SDK 不再经 bridge 下发任何账户上下文数据。SDK 通过 `addJavascriptInterface` 向页面注入名为 `UserCenter` 的桥对象(`window.UserCenter`),**仅暴露一个方法**:

| Bridge 方法 | 参数 | 返回 | 说明 |
| --- | --- | --- | --- |
| `postAccountAction(action)` | `String action` | 无 | H5 向 SDK 回传账号动作。仅接受 `logout`、`switch_account`、`session_invalid` 三个取值;其他任何取值在进入 SDK 回调前归一为 `unknown`。回调在 UI 线程派发。 |

契约约束:

- Bridge 暴露面固定为 `postAccountAction` 一个方法,不得扩大;**不提供任何账户上下文读取**(已移除 `getAccountContext`)及文件、媒体、下载、跳转或通用能力方法。
- 非法动作值不抛错、不透传,统一归一为 `unknown`,由 SDK 侧忽略或仅做诊断记录。

## 4. 账号动作语义

| 动作 | 语义 |
| --- | --- |
| `switch_account` | **切换游戏小号**(不是切换 5755 账户)。SDK 进入游戏小号选择页,由玩家明确选择目标小号后才完成切换;不展示小号自动进入提示,不允许静默自动切换到下一个小号;玩家取消切换时保持当前游戏小号,当前游戏会话不被更改。没有当前游戏小号时,不得为了切换而隐式触发初始化、登录或退出登录。 |
| `logout` | **退出登录**。SDK 清理 5755 登录态和当前游戏小号,触发账号变化通知,返回 5755 账户登录窗口;不重新展示协议告知。 |
| `session_invalid` | 登录态失效(由平台/H5 检测后回传)。**与 `logout` 同处置**:清理 5755 登录态与当前游戏小号、触发账号变化通知、返回 5755 账户登录窗口;**区别仅在触发者**(`logout` 玩家主动、`session_invalid` 平台侧检测)。bridge 只传单一 `session_invalid`、不带 reason 子类型,故按账户级失效统一回登录窗口,不在此通道区分主账户/小号失效。 |
| `unknown` | 非法动作的归一值,不触发任何账号状态变更。 |

红线:

- **用户中心不提供切换 5755 账户**。切换 5755 账户只能通过退出登录后重新登录完成。
- 三动作按「是否离开当前游戏会话」分两类:`logout` 与 `session_invalid` **都返回 5755 账户登录窗口**(并触发账号变化);`switch_account` 留在游戏内进小号选择页、**不返回登录窗口**。

## 5. 用户中心 = 平台真实 H5

- 用户中心是**平台远程 H5 页**,以**主账户为核心**(账号安全、绑定手机、订单等由平台页承载)。SDK 用 WebView 加载平台用户中心 URL,经查询参数传入 `platformToken` 供平台页拉取主账户内容。
- 用户中心 URL **不进静态协议域**(协议页走静态协议路径见 `07`/`01`;用户中心走独立、可由平台配置的 H5 地址),**由平台配置提供,SDK 不硬编码**。
- 安全:加载远程 URL 须 https;`origin` 受控(评审加载域名/来源校验,防任意页面调用 Bridge);容器禁用项(文件/媒体/APK/外跳/通用 H5)保持关闭;bridge 仅 `postAccountAction`(§3)。
- **不向游戏暴露主账户**:`account = 游戏小号 ID` 的对游戏契约不变;主账户仅在"平台对玩家"的用户中心 H5 内由平台承载,不经公开 API 或 bridge 透给游戏。
