# 01 产品范围与能力白名单

本文是 5755 Android SDK 最小化 AAR 的权威产品范围定义,规定本产品的定位、能力白名单、公开 API 白名单、明确排除项、范围边界和技术基线。所有实现、接入文档、自检和验收均以本文边界为准;任何能力进入或退出产物,都必须先修订本文。

## 1. 产品定位

5755 Android SDK 最小化 AAR 是当前唯一上线产物。它只包含最小上线链路必需的业务能力和 SDK 基础能力,不继承完整 SDK 的 Manifest、依赖和 API 面。

- **单一交付产物**:接入方通过 Gradle 只依赖一个 AAR(`5755-sdk-release.aar`)。该 AAR 由单一 `sdk` 源码模块产出,内部按包结构分层(`com.m5755.operate.core.*` 核心 / `com.m5755.sdk.ui.*` 业务配套 UI);AGP 原生 `assembleRelease` 即含全部类,不依赖 fat-aar 插件(见 docs/adr/0006)。
- **构建方式**:产物必须从 SDK 源码或模块级构建配置产出。不允许对完整 AAR 解包删类、删资源或删 so 作为生产构建方式。
- **白名单策略**:产物内容由能力白名单约束,未进入白名单的能力默认不进包。UI 物料、Manifest 组件、依赖和公开 API 都跟随能力白名单装配。
- **`account` 语义**:SDK 对外返回的 `account` 定义为**当前游戏小号 ID**。它与当前游戏关联,可用于登录态校验、角色上报和支付归属;接入开发者应将它映射为游戏内唯一用户。一个 5755 账户可以包含多个游戏小号;SDK 不把 5755 主账户 ID 暴露给游戏。5755 账户昵称只用于 SDK UI 展示,不作为 `account` 对外承诺。
- **服务端职责边界**:游戏服务端负责登录态校验、订单创建、充值回调验签、金额校验、账号归属校验和幂等发放。Android AAR 不内置这些服务端实现;客户端支付回调只作为 UI 或处理中提示,不作为物品发放依据。服务端接入只定义 HTTP/API 协议,不绑定 PHP、Java、Go、Node 或任何特定技术栈。

## 2. 能力白名单

| 能力 | 规格 |
| --- | --- |
| 协议告知 | 未完成协议确认不得完成登录。按安装级和协议版本级触发:新安装、清除数据、卸载重装或协议版本升级后展示;同一安装内协议已同意且版本未升级时不重复展示。协议正文为**静态 H5 页**,由 SDK 内置固定**协议域**加载(`p.xingninghuyu.com/agreement/*`,见 07 §2),与平台配置的用户中心 H5 **分域**;API host / 协议域 / 用户中心三类外部域用三种机制解析,见 ADR-0008。 |
| 初始化与配置 | 初始化成功后才允许调用后续业务 API。初始化必须完成配置拉取、渠道诊断、接入自检输出和维护门禁判断。初始化失败有可采集诊断输出。 |
| 接入自检 | 按最小化 AAR 白名单定义,只校验最小链路接入要求。阻断型自检项失败时 `init` 不能成功;诊断型自检项只输出诊断并降级或回退默认值,不阻断初始化、登录、角色上报或支付。 |
| 登录 | 返回当前游戏小号 ID 和登录令牌。SDK 内部在登录流程中处理协议告知、5755 账户登录或自动登录、实名认证、防沉迷进入游戏门禁、游戏小号选择或默认游戏小号自动进入提示。 |
| 账号变化 | 退出登录、踢号、5755 账户失效、游戏小号失效或切换游戏小号统一收敛到账号变化通知。防沉迷门禁阻断和维护门禁阻断不触发账号变化。 |
| 登录态校验配合 | SDK 返回当前游戏小号 ID 和登录令牌,并在角色上报、支付等归属敏感动作前做"本地已登录且令牌非空"前置检查。真正的登录态有效性由游戏服务端基于当前游戏小号 ID 和登录令牌校验,SDK 不在客户端替代服务端登录态校验。 |
| 角色上报 | 使用 `RoleInfo` 与 `sendRoleInfo`。有角色时 `roleId` 必须游戏内唯一;游戏无法区分角色时可以不调用。必填字段含区服 ID、区服名称、角色 ID、角色名称、角色等级、战力、关卡、累计充值(保留 2 位小数)和公会;确实不存在的字段传 `"-1"`。由游戏在服务端登录态校验通过、游戏进入且角色资料就绪后触发,SDK 不自动上报。 |
| 支付 | 使用 SDK 自有支付容器和服务端订单/回调协议,不引入支付宝/微信原生 SDK。支付 UI 展示和支付请求完全取自 `Order` 入参,订单绑定当前游戏小号、区服、角色、金额、商品和 CP 订单号;归属字段缺失时必须失败并输出诊断,禁止固定演示订单兜底。支付由玩家在游戏内点击购买后由游戏调用,SDK 不主动发起。 |
| 充值回调支持 | AAR 提供客户端交接口径、订单归属信息、协议文档说明、模拟回调验收和诊断支持。客户端支付状态(已发起、取消、失败、处理中)与充值回调结果强制分开;物品发放只以游戏服务端处理 5755 充值回调为准。 |
| 登出/退出/销毁 | 覆盖 `logout`、退出游戏确认(`shouldQuitGame`)和 `destroy` 的最小生命周期链路。 |
| 防沉迷 | 作为进入游戏和支付的合规门禁,分为防沉迷进入游戏门禁和防沉迷支付门禁两个独立门禁。阻断时展示明确提示并输出诊断,不触发账号变化或登出。 |
| 维护门禁 | 由平台配置驱动,SDK 在初始化成功后、协议告知或自动登录前展示提示并阻断进入流程。它不同于 SDK 代码维护或强制更新,阻断不触发账号变化。 |
| 用户中心 H5 | 通过悬浮球入口打开最小用户中心 H5 容器,由 Android 系统 WebView 和最小 JS Bridge 承载。H5 **不经 bridge 读取账户上下文**(主账户内容由平台页凭 `platformToken` 自取),仅把退出登录、切换游戏小号或账号失效类动作回传 SDK;不维护另一套当前账号,不提供切换 5755 账户。悬浮球只保留用户中心入口。 |
| 渠道读取诊断 | 运行时只读渠道,支持 manifest meta-data 与 v2/v3 APK Signing Block 两种来源。渠道异常一律回退 `default`,不阻断最小链路。 |
| SDK 基础能力 | 轻量网络、轻量页面容器、轻量键值存储、协议级安全和内部诊断输出。诊断输出不是接入方业务 API,但必须稳定、可采集,覆盖初始化、自检、渠道、维护、协议、5755 账户、实名、防沉迷、游戏小号、登录态校验配合、角色上报、支付和用户中心。 |

## 3. 公开 API 白名单

公开 API 面限定在 `com.m5755.operate.api.*` 白名单,只保留最小上线链路需要的方法和字段。范围外旧入口不做空实现兼容——接入方若调用完整包能力,应在编译期暴露问题。

承诺的公开类:

- `com.m5755.operate.api.Operate`
- `com.m5755.operate.api.Options`
- `com.m5755.operate.api.User`
- `com.m5755.operate.api.UserListener`
- `com.m5755.operate.api.RoleInfo`
- `com.m5755.operate.api.RoleMeta`
- `com.m5755.operate.api.Order`
- `com.m5755.operate.api.Listener`
- `com.m5755.operate.api.DataListener`
- `com.m5755.operate.api.OnQuitGameListener`(`shouldQuitGame` 的回调参数类型)
- `com.m5755.operate.provider.OperateCode`

| API 面 | 保留项 |
| --- | --- |
| SDK 版本与启动 | `getVersion`、`onGameStart` |
| 初始化 | `init`、必要 `Options` 字段 |
| 用户监听 | `setUserListener`、`getUserListener`、`UserListener.onLogout` |
| 登录与账号 | `login`、`isLogin`、`getUser`、`changeUser`、`logout` |
| 用户数据 | `User.getAccount`、`User.getToken` |
| 角色 | `RoleInfo`、`sendRoleInfo` |
| 支付 | `Order`、`recharge` |
| 退出与销毁 | `shouldQuitGame`、`destroy` |

必须移除或不暴露的 API:平台 App 检查/跳转、客服/IM、活动/优惠券、强更、广告归因、设备标识采集或透传、OAID/Android ID 透传 key(`Options.KEY_OAID`、`Options.KEY_ANDROID_ID`)、加速、连点器、通用 H5 文件/媒体/APK/外跳能力,以及 `showActivityScheme`、`isVendingAppInstalled`、`setServer`。

业务配套 UI 由 SDK 核心状态机驱动;接入方只能通过公开 API 白名单触发业务流程,不直接调用 `SdkUi` 或其他 UI 演示入口。

## 4. 明确排除项

### 4.1 依赖黑名单

最小化 AAR 的依赖清单不得包含以下项,除非有白名单能力证明必须依赖并留下显式评审记录:

- 网络:Volley、OkHttp、任何接入方网络库绑定
- 兼容框架:support-v4、AndroidX
- 语言运行时与序列化:kotlin-stdlib、protobuf
- 设备与归因:OAID、GDTActionSDK、AppConvert、广告归因 SDK
- 监控:monitorsdk/Kwai、任何第三方监控或分析 SDK
- native:shadowhook、pine、任何 `.so`(AAR 内不得出现 `libs/`、`jni/`)
- 支付:支付宝原生 SDK、微信原生 SDK 及其他第三方支付原生 SDK

### 4.2 能力排除

以下能力不进入产物,对应 UI 资产即使已存在也不装配进当前 AAR:

- 平台 App 下载、安装、检查或跳转流程
- 客服、IM、聊天 Provider 或 IM 存储
- 活动、优惠券、营销工具、福利、消息、饭团充值、平台币(其中「平台币 / 平台余额 / 余额支付」的范围外·可拓展定位与合规定性见 §5 与 ADR-0012)
- 强更或版本更新门禁
- 广告归因、第三方监控、设备标识采集或透传
- 加速、连点器、native hook、native 安全、引擎 hook
- 通用 H5 容器能力:文件上传、媒体选择、APK 下载/安装、外部 App 跳转
- 将渠道标识符暴露为游戏侧 API,或允许接入方通过透传参数覆盖渠道
- v1 ZIP comment 免重签渠道读取(不作为运行时承诺);渠道写入工具不进入运行时产物

### 4.3 Manifest 排除

最小化 Manifest 由能力白名单生成,不继承完整 SDK Manifest。只保留最小链路必需权限(网络访问)、最小页面容器所需 Activity 和运行必需 meta-data;Provider、Service、Receiver 和 queries 默认不保留。必须排除:IM Provider、FileProvider、下载/安装权限、平台 App/支付宝/微信/客服等 queries、客服/活动/强更组件、可选 native hook 组件和广泛 package queries。

## 5. 范围边界与未来项

> 分期词汇已统一为里程碑 M1–M5 + GA(见 `docs/00-roadmap.md`),「一期/二期」退役。本节列出当前产物边界:已交付的生产化、移交 GA 受控验收的真实链路、以及明确在 v2 范围外的未来/可选能力。范围外能力进入产物前,必须重新走能力白名单、公开 API、Manifest、依赖和接入自检评审。

| 事项 | 状态 |
| --- | --- |
| 生产 AAR 纯净化与运行模式隔离(`verifyPublicAarPurity` 五维 + 元测试、build-tag 排除 dev 能力) | **M4 已交付** |
| 真实平台配置、5755 账户、游戏小号、支付查询、充值回调线上链路联调 | **GA 受控验收**(见 `production-controlled-acceptance-handoff.md`) |
| 真实用户中心 H5 联调(平台侧 H5 容器与 URL 契约) | v2 范围外·未来 |
| 设备标识采集或透传(OAID/Android ID) | v2 范围外·未来 |
| 广告归因 | v2 范围外·未来 |
| 第三方监控(monitorsdk/Kwai) | v2 范围外·未来 |
| 平台余额 / 余额支付(平台币体系):玩家预存真实货币、下单从余额扣减 | **v2 范围外·可拓展**(门开着、非永久排除;v2 不自持余额、走持牌机构直连代收;须过 §5 评审门 + 合规前置〔申牌 vs 单用途、备付金/二清、未成年人限额前移〕;定性与前向兼容缝见 ADR-0012) |
| 客服/IM/protobuf 消息 | 可选·永久排除(§4.2) |
| 平台 App 下载、安装、跳转 | 可选·永久排除(§4.2) |
| 活动、优惠券 | 可选·永久排除(§4.2) |
| 强更/版本更新门禁 | 可选·永久排除(§4.2) |
| 加速、连点器、native hook | 可选·永久排除(§4.2) |
| 文件上传、媒体选择、APK 处理、外部 App 跳转 | 可选·永久排除(§4.2) |

范围外能力的后续联调不引入完整包能力、不扩大公开 API、不改变 `account = 当前游戏小号 ID` 的语义。

## 6. 技术基线

| 项目 | 规格 |
| --- | --- |
| Android 运行基线 | `minSdkVersion 21`,不因最小化重构提高最低系统版本 |
| 游戏包规范 | 游戏包须满足 `targetSdk >= 26`,并满足 32/64 位 ABI 规范和签名规范(由接入自检校验,与 SDK 运行支持范围区分);游戏 Activity 须声明 `android:configChanges="orientation|screenSize|smallestScreenSize|screenLayout"`(手游引擎导出模板默认配置;SDK 层依赖它在旋转时保留业务状态,未声明时接入自检输出诊断警告,不阻断) |
| 构建基线 | JDK 8+;Gradle 7.0+ 推荐但非硬门槛 |
| 接入方式 | Gradle AAR 接入;在线 AAR 优先,本地原样 AAR/JAR 备选;不要求手动解包 jar、资源或 native 库 |
| 网络 | SDK 轻量网络层,基于 Android 平台能力实现;覆盖异步执行、超时、有限重试、必要取消、响应解析、错误归一和诊断日志 |
| 页面 | SDK 轻量页面容器,承载协议告知、登录、实名认证、防沉迷提示、维护提示、游戏小号选择、小号自动进入提示、支付容器和用户中心 |
| 用户中心 H5 | Android 系统 WebView + 最小 JS Bridge;Bridge 仅 `postAccountAction` 回传账号动作(不读取账户上下文,`getAccountContext` 已移除);主账户内容由平台页凭 `platformToken` 自取 |
| 存储 | SDK 轻量键值存储,只保存登录态标记、配置缓存、渠道诊断和必要运行状态;不使用数据库或大文件缓存 |
| 安全 | 协议级安全能力:HTTPS、签名、登录 token/session、支付归属校验、回调校验支持和诊断;不引入 native 加密库或第三方安全 SDK |
| 日志诊断 | SDK 内部日志、错误码、有界状态快照和自检结果;每个最小链路失败都有错误码和有界状态快照 |

### 渠道标识符规格

| 项目 | 规格 |
| --- | --- |
| 读取来源 | Manifest meta-data(`m5755_channel`、`m5755.channel`、`com.m5755.channel`、`channel`)、APK Signing Block(`0x71777777`、`0x57550001`) |
| 归一化 | 1-64 个 ASCII 字符;只允许字母、数字、下划线、短横线、点号;统一小写 |
| 双来源一致性 | manifest 与 Signing Block 同时存在时,归一后必须一致;不一致解析为 `default`,原因 `source_mismatch` |
| 回退 | 缺失、不可读、格式非法、来源不一致或与运营配置不一致时解析为 `default`,不阻断初始化、登录、角色上报或支付 |
| 诊断字段 | `manifestChannelRaw`、`signingBlockChannelRaw`、`resolvedChannel`、`reason`(`missing`/`unreadable`/`invalid_format`/`source_mismatch`/`operations_mismatch`) |
