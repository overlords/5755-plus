# 04 平台网关 API(JSON 契约)v2

本文档定义 5755 Android SDK 内部平台网关与平台服务端之间的 HTTP JSON 契约。这些接口由 SDK 内部网关发起,不向接入游戏暴露;接入方只使用 `com.m5755.operate.api.*` 公开白名单 API。

本契约由**平台服务端**实现:与 SDK 同仓开发、同步演进的服务端组件,v2 范围只含 SDK 网关面(SDK-facing 端点、充值回调、dev 控制面)及支撑它们的最小数据模型(含实名/年龄/维护状态)。`/api/sdk/v1/*` 路径属旧平台原型(U10)的现行对外契约,由其继续承担,**不在本契约内**;本契约(v2)与其域名、路径互不重叠(见第 6 节环境矩阵)。

字段口径(全文适用):

- `account` 固定表示当前**游戏小号** ID,任何接口都不得用它表示 5755 账户(平台主账户)ID。
- 5755 账户使用 `platformAccountId` / `platformToken`(平台主账户登录令牌)。
- 游戏小号登录令牌使用 `token`,仅由游戏小号登录接口签发。
- **充值回调(服务端)**指平台服务端到游戏服务端的发货通知;**客户端支付回调**仅作为 UI 或处理中提示,不作为物品发放依据。

## 1. 统一约定

### 1.1 路径与传输(资源式设计)

SDK 契约面收敛为 **9 条资源式路径**,以 HTTP 方法区分语义:`GET` 读取、`POST` 创建/提交、`PUT` 幂等设置。全部路径挂在 `/api/sdk/v2/` 下:

| 路径 | 方法 | 用途 | 替代的动作式端点 |
| --- | --- | --- | --- |
| `/api/sdk/v2/config` | GET | 初始化配置拉取 | `init` |
| `/api/sdk/v2/sms-codes` | POST | 短信验证码请求 | `sms` |
| `/api/sdk/v2/account-sessions` | POST | 5755 账户登录 | `login` |
| `/api/sdk/v2/account-sessions` | GET | 5755 账户有效检查 | `account/validate` |
| `/api/sdk/v2/real-name` | GET | 实名认证检查 | `real-name/verify` |
| `/api/sdk/v2/real-name` | POST | 实名认证提交 | `real-name/submit` |
| `/api/sdk/v2/subaccounts` | GET | 游戏小号列表 | `subaccounts/list` |
| `/api/sdk/v2/subaccounts` | POST | 添加游戏小号 | `subaccounts/create` |
| `/api/sdk/v2/subaccounts/default` | PUT | 设置默认游戏小号 | `subaccounts/set-default` |
| `/api/sdk/v2/subaccount-sessions` | POST | 游戏小号登录 | `subaccounts/login` |
| `/api/sdk/v2/subaccount-sessions` | GET | 游戏小号登录态校验 | `oauth/check` |
| `/api/sdk/v2/roles` | PUT | 角色上报(upsert) | `role/report` |
| `/api/sdk/v2/orders` | POST | 支付创建 | `payment/orders` |
| `/api/sdk/v2/orders/{orderId}` | GET | 订单查询 | `payment/orders/{orderId}` |

传输约定:

- 传输使用 HTTPS。SDK 对包内配置的裸域名运行时默认补 `https://`。
- `POST` / `PUT` 请求体使用 JSON,`Content-Type: application/json; charset=UTF-8`;`GET` 无请求体,业务参数走 query string,凭据走请求头(见 1.4)。所有请求携带 `Accept: application/json`。
- 旧协议(`/sdk/...` 路径、form-urlencoded 请求、properties 文本响应)不在本契约内,不得回流。
- **`login/complete`、`subaccounts/create-first`、登录请求的 `platformAccountId` 旁路在 v2 中不存在对应路由或行为**,不属于"废弃保留",而是从路由层面不注册。

### 1.2 统一响应 ApiResult

所有响应统一为 JSON `ApiResult`:

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `success` | boolean | 业务是否成功。 |
| `code` | int | 业务码;SDK 失败时归一到现有公开 `OperateCode`,不新增公开错误码。 |
| `message` | string | 业务信息;保留后端业务原因,面向人读,不作为机器分流依据。 |
| `reason` | string | **机器可读失败原因枚举**(见 1.2.1)。失败响应(`success=false` 或 `data.valid=false`)必填;成功响应可省略。属 SDK 内部契约,不是公开 API,不进入公开回调字段。 |
| `data` | object | 成功接口按能力返回的数据体。 |

失败处理规则:

- 后端业务失败可使用非 2xx HTTP 状态,但响应体仍必须保持 `ApiResult` 形态;SDK 必须读取失败响应体中的 `code/message/reason`,不得因 HTTP 状态丢失业务原因。
- HTTP 成功且 `success=true` 仅表示接口调用成功,不等于业务放行(详见各端点业务规则,如会话检查类端点的 `data.valid`)。
- SDK 状态机的回退分支只依赖 `reason`(及 `data.valid`),不解析 `message` 文本;公开回调仍归一到 `OperateCode`,细分原因进入 `message` 与内部诊断。
- JSON 解析必须忽略后端新增的未知字段;缺少 SDK 必需字段时必须明确失败并输出可排查诊断。
- 响应 JSON 不可解析时按失败处理。

#### 1.2.1 reason 枚举

`reason` 枚举按《03 进入与账号流程》第 3 节阻断/回退表逐行推导,服务端取值与 SDK 状态机回退动作一一对应:

| reason | 触发情形 | SDK 回退动作(对齐 03 §3) | 触发账号变化 |
| --- | --- | --- | --- |
| `maintenance` | 游戏维护中;维护期间登录类请求被服务端拒绝 | 展示维护提示,阻断进入流程;不得误判为账号失效(03 §2.2) | **否** |
| `credential_invalid` | 密码错误、登录凭据无效 | 停留 5755 账户登录窗并提示,可重试(03 §2.4) | 否 |
| `sms_code_invalid` | 验证码错误或尝试次数超限 | 停留登录窗提示验证码错误,可重新输入或重新获取 | 否 |
| `sms_code_expired` | 验证码已过期 | 停留登录窗提示重新获取验证码 | 否 |
| `sms_rate_limited` | 短信验证码请求过于频繁 | 停留登录窗提示稍后再试,不阻断其他链路 | 否 |
| `platform_account_invalid` | 5755 账户失效:令牌无效/过期、踢号、账号停用、账号不存在 | **清理当前游戏小号,返回 5755 账户登录窗口**(03 §2.5、§3) | **是** |
| `subaccount_invalid` | 游戏小号失效:小号不存在、停用、小号令牌无效/过期 | **进入游戏小号选择页,不返回 5755 账户登录窗口**(03 §3) | **是** |
| `real_name_required` | 5755 账户未实名,触发需实名的动作 | 进入实名认证提交流程,通过后继续小号流程(03 §2.6) | 否 |
| `anti_addiction_entry_blocked` | 防沉迷进入游戏门禁阻断 | 展示防沉迷提示,阻断进入流程;不回登录窗或小号选择(03 §2.7) | **否** |
| `anti_addiction_payment_blocked` | 防沉迷支付门禁阻断(含服务端复核命中) | 仅失败本次支付并提示限制,保持登录态(03 §2.7、05 §4) | **否** |
| `subaccount_limit_reached` | 当前游戏下小号数量已达 10 个 | 停留小号选择页,仅提示上限原因,不允许继续添加(03 §4.2) | 否 |
| `order_invalid` | 订单字段缺失/非法、订单不存在、订单归属不符 | 仅失败本次支付/查询并输出诊断,不改变账号与小号状态 | 否 |
| `param_invalid` | 请求字段缺失或格式非法(如 `roleId="-1"`) | 仅失败本次调用并输出可采集诊断,不发起回退跳转 | 否 |
| `signature_invalid` | 验签失败:缺签名头、keyId 未知、签名不匹配 | 按平台不可用阻断并输出诊断;不得误判为账号失效 | 否 |
| `timestamp_expired` | 签名时间戳超出 ±300 秒窗口 | 按平台不可用阻断,诊断提示校时后重试 | 否 |
| `platform_unavailable` | 平台内部错误、依赖不可用等接口级失败 | 明确阻断并提示稍后重试;**不得用本地态放行,也不误判为账号失效**(03 §2.4) | 否 |
| `device_verification_required` | 设备首次密码登录需短信验证(里程碑 3) | 进入设备安全验证页,验证通过后自动续登;不触发账号变化 | 否 |

约束:

- 5755 账户失效与游戏小号失效是两类独立回退路径(03 §3 硬规则),服务端必须用 `platform_account_invalid` / `subaccount_invalid` 严格区分,**不得混用**。
- 服务端新增 reason 取值时 SDK 必须按未知 reason 处理:归入 `platform_unavailable` 同等的保守阻断分支并输出诊断,不得崩溃或误放行。

### 1.3 请求头与签名

每个请求携带产物标识请求头与签名请求头,取值来自 AAR 包内平台配置(见第 6 节):

| 请求头 | 必带 | 说明 |
| --- | --- | --- |
| `X-M5755-Artifact-Type` | 是 | 产物类型:`integration` / `production` / 构建验收用 `local`。 |
| `X-M5755-Platform-Env` | 是 | 平台环境:`dev` / `prod` / 构建验收用 `local`。 |
| `X-M5755-Key-Id` | 是 | 签名 key 的非密标识,服务端据此选择验签密钥。 |
| `X-M5755-Timestamp` | 是 | 请求发起的 Unix 时间戳(秒),参与签名与防重放窗口。 |
| `X-M5755-Signature` | 是 | 请求签名,HMAC-SHA256 十六进制小写。 |

签名算法:

- 算法:`HMAC-SHA256(secret, canonical)`,`secret` 为 `keyId` 对应的签名密钥。
- canonical 串构造(各段以 `\n` 连接):

```
HTTP方法(大写) + "\n"
+ 路径(如 /api/sdk/v2/config) + "\n"
+ 规范化 query(参数按键字典序排序后 key=value 以 & 连接;无 query 为空串) + "\n"
+ X-M5755-Timestamp 值 + "\n"
+ 请求体原文(GET 为空串)
```

验签与防重放规则:

- 时间戳窗口 **±300 秒**,超窗拒绝,`reason=timestamp_expired`。
- 缺任一签名头、`keyId` 未知或签名不匹配,拒绝,`reason=signature_invalid`。
- **`/api/sdk/v2/*` 全端点强制验签,dev 与生产同样开启**,不存在"联调免签"形态;dev 环境提供公开测试密钥便于联调与回归。
- 密钥由 SDK 构建或发布配置固定;`keyId`、签名配置版本(`signatureConfigVersion`)等只允许出现非密标识;诊断与日志禁止输出签名密钥、签名原文等完整签名材料。
- 环境、证书、签名配置、keyId、日志级别和平台 host 只能由 SDK 构建或发布配置固定,不能由接入游戏通过初始化参数、Manifest、H5、透传或运行时开关切换。

### 1.4 GET 凭据请求头

`GET` 请求的业务参数走 query string,但**敏感凭据禁止进入 query string**(避免进入访问日志、代理缓存与 Referer),统一走自定义请求头:

| 请求头 | 说明 |
| --- | --- |
| `X-M5755-Platform-Token` | 5755 账户登录令牌(`platformToken`),账户/实名/小号类 GET 端点携带。 |
| `X-M5755-Token` | 当前游戏小号登录令牌(`token`),小号登录态校验与订单查询携带。 |

- `platformAccountId`、`account`、`gameId` 等非凭据标识仍走 query。
- 凭据请求头参与 1.3 签名的请求体段以外的部分不变;canonical 串不含请求头,凭据有效性由服务端按会话校验。
- `POST` / `PUT` 端点的凭据沿用请求体字段(`platformToken` / `token`),不重复走请求头。

### 1.5 超时与重试

| 项 | 取值 |
| --- | --- |
| 连接超时 | 5 秒 |
| 读取超时 | 5 秒 |
| 单次请求总等待 | 9 秒,超出按超时失败(`OperateCode.TIMEOUT`) |
| 自动重试 | 网关不做自动重试;失败归一到 SDK 回调,由业务流程决定后续动作 |

### 1.6 公共请求字段

| 字段 | 说明 |
| --- | --- |
| `gameId` | 当前游戏 ID,5755 平台分配。 |
| `platformAccountId` | 5755 账户 ID;实名和小号类接口必传。 |
| `platformToken` | 5755 账户登录令牌;实名和小号类接口必传(GET 走 `X-M5755-Platform-Token` 请求头)。 |
| `account` | 当前游戏小号 ID,按接口需要传入。 |
| `token` | 当前游戏小号登录令牌,按接口需要传入(GET 走 `X-M5755-Token` 请求头)。 |
| `channelId` | SDK 解析后的渠道标识符,用于 5755 账户渠道归因;不允许游戏侧透传覆盖。 |
| `channelSource` | 渠道字段来源,如 Manifest、APK Signing Block 或默认值。 |

### 1.7 诊断与敏感信息

- SDK 诊断可记录:`requestId`、`baseHost`、`platformEnv`、`artifactType`、`configVersion`、`signatureConfigVersion`、`keyId`、`reason` 等非密字段,用于与平台日志对齐。
- 诊断、日志与验收报告禁止记录:完整验证码(含 mock `devCode`)、密码、完整 `credential`、明文实名资料、签名密钥与完整签名材料、完整 token。登录账号只保留掩码。

## 2. 端点契约

请求字段表中"位置"列标注字段来源:`query`(GET 查询串)、`body`(JSON 请求体)、`header`(自定义请求头)、`path`(路径参数)。

### 2.1 初始化配置 `GET /api/sdk/v2/config`

用途:拉取初始化配置、维护门禁、游戏级防沉迷开关、协议版本与诊断字段。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | query | 是 | 当前游戏 ID。**缺失或游戏不存在时服务端失败,SDK 阻断初始化**(阻断型自检项)。 |
| `sdkVersion` | query | 是 | SDK 版本,参与 `updateRequired` 计算。 |
| `packageName` | query | 是 | 宿主包名。缺失时服务端不阻断,SDK 仅输出诊断(诊断型自检项)。 |
| `channelId` | query | 是 | 渠道标识符。缺失或非法时回退 `default` 并输出诊断,不阻断。 |
| `channelSource` | query | 是 | 渠道来源。同上,仅诊断。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `gameId` | 游戏 ID。 |
| `gameName` | 游戏名称。 |
| `maintenance` | 维护门禁对象:`enabled`(是否维护中)、`message`(维护提示)。 |
| `antiAddictionEntryBlocked` | 防沉迷进入游戏门禁(**游戏级开关与维护级状态**,见业务规则)。 |
| `antiAddictionPaymentBlocked` | 防沉迷支付门禁(同上,游戏级)。 |
| `protocolVersion` | 协议告知版本。 |
| `requestId` | 排障诊断 ID。 |
| `configVersion` | 平台配置版本。 |
| `sdkLatestVersion` / `sdkMinVersion` / `updateRequired` | SDK 版本信息与是否强制更新。 |
| `loginDomain` / `paymentDomain` | 登录域、支付域。 |

业务规则:

- 维护门禁、防沉迷开关、协议版本、`requestId`、`configVersion` 必须来自真实平台配置,不允许本地默认成功配置放行,也不允许服务端硬编码常量伪装真实配置。
- **防沉迷字段语义分层**:本端点返回的 `antiAddictionEntryBlocked` / `antiAddictionPaymentBlocked` 是**游戏级开关与维护级状态**(平台可对整个游戏一刀切);**账户级防沉迷判定以 `/real-name` 端点返回为准**(见 2.4),SDK 运行态门禁以后者覆盖前者。两处任一为 `true` 即阻断对应动作。
- `updateRequired` 计算口径:`compareVersion(sdkVersion, sdkMinVersion) < 0` 时为 `true`,即请求 `sdkVersion` 低于平台配置的最低版本即要求强制更新。
- 本端点不返回 `accountNickname`:初始化阶段尚无 5755 账户上下文,账户昵称由登录与实名类端点返回。
- 真实初始化失败时必须阻断进入流程,失败状态可观察,中间态不得伪装成业务成功。
- `passthrough` 透传字段已废弃,生产 SDK 不发送;不允许用透传控制环境、fixture 或业务链路。

### 2.2 短信验证码请求 `POST /api/sdk/v2/sms-codes`

用途:玩家在验证码登录页点击"发送验证码"时,请求平台向登录页输入的手机号发送一次验证码。该接口只用于取得本次 5755 账户登录凭据,不表示登录成功、注册入口或设备验证;发生在验证码登录提交之前。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `loginAccount` | body | 是 | 登录页输入的**手机号**(登录标识);本端点只接受可接收短信的手机号格式,不接受其他账号形态。不得使用游戏小号 `account` 字段。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `codeId` | 服务端生成的本次短信请求标识,用于排障和限流关联。 |
| `loginAccountMasked` | 脱敏手机号;不返回完整手机号。 |
| `expiresAt` | 验证码有效期。 |
| `providerMode` | 短信服务模式,如 `mock` 或真实短信供应商模式。 |
| `providerStatus` | 短信服务回执或受理状态。 |
| `devCode` | 仅 mock 模式允许返回,供联调 UI 提示。 |

业务规则:

- **生产短信模式不得返回明文验证码**;`devCode` 仅限联调,且不得进入 SDK 诊断或验收报告。
- 同号短窗重复请求按限流拒绝,`reason=sms_rate_limited`;dev/mock 模式可放宽限流以便异常路径回归。
- 验证码错误与过期分别返回 `sms_code_invalid` / `sms_code_expired`(由登录端点消费时返回)。

### 2.3 5755 账户会话 `/api/sdk/v2/account-sessions`

#### 2.3.1 账户登录 `POST /api/sdk/v2/account-sessions`

用途:5755 账户登录的唯一入口。SDK 不提供单独注册通道;验证码登录和密码登录都是登录方式,账号是否存在、是否需要创建 5755 账户由服务端识别并处理。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `loginMethod` | body | 是 | 登录方式:`sms` 或 `password`。 |
| `loginAccount` | body | 是 | 玩家输入的手机号或账号。不得使用游戏小号 `account` 字段。 |
| `credential` | body | 是 | 验证码或密码等登录凭据。SDK 不得在日志、诊断或错误信息中输出完整 `credential`。 |
| `channelId` | body | 是 | SDK 解析后的渠道标识符。 |
| `channelSource` | body | 是 | 渠道来源。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `platformAccountId` | 5755 账户 ID。 |
| `platformToken` | 5755 账户登录令牌。 |
| `displayName` | 展示名,仅用于 SDK UI。 |
| `gameEntry` | 首次入场信息:`isNewGameUser`(是否首次进入当前游戏);`createdSubaccount`(服务端创建的首个游戏小号:`account`、`gameId`、`platformAccountId`、`displayName`、`isDefault=false`)。 |
| `expiresAt` | 登录态有效期。 |

业务规则:

- `platformAccountId/platformToken` 表示 5755 账户登录态,**不能占用 `account/token`**;本接口不返回游戏小号 `account/token`。
- **请求不接受 `platformAccountId` 字段**:不存在"指定主账户 ID 跳过凭据校验"的旁路;`login/complete` 路由不存在。
- **`loginMethod=password` 必须执行真实密码校验**,凭据错误返回 `reason=credential_invalid`;不允许空实现放行。`loginMethod=sms` 消费验证码,错误/过期分别返回 `sms_code_invalid` / `sms_code_expired`。
- 维护中服务端拒绝登录,`reason=maintenance`;SDK 按维护门禁阻断,不触发账号变化。
- 首个游戏小号属于服务端行为:新 5755 账户首次进入当前游戏时,服务端在本接口内保障首个真实游戏小号存在(`isDefault=false`),并随小号列表(2.5)返回;**小号名称由平台生成**(如"小号1"风格的递增命名),SDK 不调用任何 `create-first` 类接口,也不本地命名。
- `channelId/channelSource` 用于 5755 账户新用户渠道归因;老用户后续登录不得覆盖既有归因。
- 登录失败时 SDK 必须能向调用方呈现后端 `code/message`,并依据 `reason` 区分凭据错误、账号无效、维护与平台不可用。

#### 2.3.2 账户有效检查 `GET /api/sdk/v2/account-sessions`

用途:SDK 自动登录或登录后检查 5755 账户是否仍有效。本地缓存的 5755 登录态只能触发自动登录或本检查,不能替代真实检查。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | query | 是 | 当前游戏 ID。主账户会话绑定 `gameId`,检查按三元组匹配。 |
| `platformAccountId` | query | 是 | 5755 账户 ID。 |
| `X-M5755-Platform-Token` | header | 是 | 5755 账户登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `valid` | 5755 账户登录态是否有效。 |
| `platformAccountId` | 5755 账户 ID。 |
| `displayName` | 展示名。 |

业务规则:

- **语义分层(全契约会话检查类端点统一)**:接口调用成功即 `success=true`,有效性看 `data.valid`:
  - `data.valid=false`(`reason=platform_account_invalid`)→ 明确失效:SDK 清理当前游戏小号态,返回 5755 账户登录窗口,触发账号变化(03 §2.5);
  - `success=false` → 接口或平台失败(`platform_unavailable` / `signature_invalid` 等):SDK 阻断并提示,**不误判为账户失效**,不能用本地 token 放行。
- 主账户会话绑定 `gameId`:令牌、账户 ID 与游戏 ID 三者必须匹配同一会话。

### 2.4 实名认证 `/api/sdk/v2/real-name`

实名认证归属于 5755 账户,不归属于游戏小号。

#### 2.4.1 实名认证检查 `GET /api/sdk/v2/real-name`

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | query | 是 | 当前游戏 ID。 |
| `platformAccountId` | query | 是 | 5755 账户 ID。 |
| `X-M5755-Platform-Token` | header | 是 | 5755 账户登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `verified` | 是否已实名。 |
| `adult` | 是否成年(按实名身份信息判定)。 |
| `antiAddictionEntryBlocked` | 防沉迷进入游戏门禁(**账户级判定,运行态以此为准**)。 |
| `antiAddictionPaymentBlocked` | 防沉迷支付门禁(账户级判定)。 |

#### 2.4.2 实名认证提交 `POST /api/sdk/v2/real-name`

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `platformAccountId` | body | 是 | 5755 账户 ID。 |
| `platformToken` | body | 是 | 5755 账户登录令牌。 |
| `realName` | body | 是 | 真实姓名。 |
| `idNumber` | body | 是 | 身份证号。 |

响应 `data` 字段:与 2.4.1 一致(`verified`、`adult`、`antiAddictionEntryBlocked`、`antiAddictionPaymentBlocked`),只返回脱敏实名状态,不回显明文实名资料。

业务规则(两方法共用):

- 未实名时(检查返回 `verified=false`,或需实名动作返回 `reason=real_name_required`)进入实名提交流程,实名通过后继续小号流程,玩家不需要重复登录。
- SDK 必须消费本端点返回的防沉迷门禁(写入运行态),不只依赖初始化配置:`antiAddictionEntryBlocked=true` 阻断进入游戏(`reason=anti_addiction_entry_blocked`);`antiAddictionPaymentBlocked=true` 阻断支付(`reason=anti_addiction_payment_blocked`),但保持当前游戏小号登录态,不触发登出或账号变化通知。
- 账户级门禁最小上线推导口径:未实名 → 阻断进入与支付;已实名未成年 → 放行进入、阻断支付;时段/时长类防沉迷限制列为 v2 范围外,不在本契约承诺内。
- **实名核验分级**:dev/联调环境实名提交为**格式校验 + mock 通过**(身份证号格式与出生日期合法即通过);**生产环境必须对接真实实名核验源,该项列入发布门禁**,不阻塞 v2 开发,但属显式合规义务,不得以格式校验冒充生产核验。
- **已实名账户锁定**:已实名的 5755 账户重复提交按**幂等成功**处理,不改值;变更实名走人工/客服流程,不在本契约内。
- SDK 诊断禁止记录明文实名资料;服务端只存储脱敏值。

### 2.5 游戏小号 `/api/sdk/v2/subaccounts`

#### 2.5.1 游戏小号列表 `GET /api/sdk/v2/subaccounts`

用途:返回当前 5755 账户在当前游戏下的真实游戏小号列表,驱动小号选择页。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | query | 是 | 当前游戏 ID。 |
| `platformAccountId` | query | 是 | 5755 账户 ID。 |
| `X-M5755-Platform-Token` | header | 是 | 5755 账户登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `defaultAccount` | 默认游戏小号 ID;未设置时为空。 |
| `subaccounts[]` | 小号数组,每项至少包含 `account`(游戏小号 ID)、`displayName`(展示名)、`isDefault`(是否默认)。 |

业务规则:

- 小号选择页只展示本接口返回的真实小号;演示小号、本地小号、本地默认列表不得进入成功路径。
- 首个小号由平台在账户登录(2.3.1)内保障创建并通过本接口返回;**已登录账户的列表为空属于平台侧异常**:SDK 阻断登录并输出诊断,不在客户端补建、不调用任何创建兜底接口。
- 只有真实列表中存在默认小号时,SDK 才展示轻量自动进入提示;没有默认小号时进入完整小号选择页。上次游戏小号不作为自动进入依据。

#### 2.5.2 添加游戏小号 `POST /api/sdk/v2/subaccounts`

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `platformAccountId` | body | 是 | 5755 账户 ID。 |
| `platformToken` | body | 是 | 5755 账户登录令牌。 |

响应 `data` 字段(新增小号,扁平结构):

| 字段 | 说明 |
| --- | --- |
| `account` | 新增游戏小号 ID。 |
| `displayName` | 新增小号展示名,平台生成。 |
| `isDefault` | 恒为 `false`:新增小号不自动成为默认小号。 |

业务规则:

- **服务端强制小号上限 10 个**:当前 5755 账户、当前游戏下已有 10 个小号时创建失败,`reason=subaccount_limit_reached`;SDK 停留选择页只提示上限(03 §4.2)。
- **服务端忽略请求中的任何 `isDefault` 字段**,新增小号恒非默认;默认身份只能由玩家通过 2.6 显式设置。SDK 也不发送该字段。
- 小号名称由平台生成,SDK 不传名称、不本地命名。
- 添加成功后 SDK 刷新小号列表;不自动进入游戏,不自动设为默认。

### 2.6 设置默认游戏小号 `PUT /api/sdk/v2/subaccounts/default`

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `platformAccountId` | body | 是 | 5755 账户 ID。 |
| `platformToken` | body | 是 | 5755 账户登录令牌。 |
| `account` | body | 是 | 要设为默认的当前游戏小号 ID。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `account` | 游戏小号 ID。 |
| `defaultAccount` | `true`,设置结果。 |

业务规则:

- **本端点幂等**:目标小号已是默认时重复设置返回成功且状态不变;同账户同游戏下默认小号互斥,设置目标默认即清除其他小号的默认标记。
- 只有玩家点击小号行上的默认标签才调用本接口;点击小号行进入游戏不等于设置默认。
- 目标小号不存在或已失效返回 `reason=subaccount_invalid`。

### 2.7 游戏小号会话 `/api/sdk/v2/subaccount-sessions`

#### 2.7.1 游戏小号登录 `POST /api/sdk/v2/subaccount-sessions`

用途:使用玩家选择或默认的游戏小号换取当前游戏小号登录令牌。**SDK 最终返回给游戏的 `account/token` 只来自本接口**。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `platformAccountId` | body | 是 | 5755 账户 ID。 |
| `platformToken` | body | 是 | 5755 账户登录令牌。 |
| `account` | body | **是** | 玩家选择或默认的游戏小号 ID。**不允许省略**:不存在"省略 account 登录默认小号"的形态,默认小号的 `account` 由 SDK 从小号列表取得后显式传入,杜绝静默默认登录(03 §4.5)。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `account` | 当前游戏小号 ID。 |
| `token` | 游戏小号登录令牌。 |
| `expiresAt` | 有效期。 |

业务规则:

- 本接口返回的 `account/token` 是游戏服务端登录态校验、角色上报和支付归属的唯一凭据。
- 本地默认游戏小号只能触发自动进入提示的前置流程,不能替代真实小号列表或本接口登录。
- **失败分流必须可区分**(03 §3 硬规则):小号不存在/停用 → `reason=subaccount_invalid`,SDK 进入小号选择页;主账户失效 → `reason=platform_account_invalid`,SDK 清理小号回 5755 账户登录窗。两者不得混用同一 reason。

#### 2.7.2 游戏小号登录态校验 `GET /api/sdk/v2/subaccount-sessions`

用途:校验当前游戏小号登录态(不是 5755 账户登录态);游戏服务端与 SDK 诊断使用同一小号令牌语义。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | query | 是 | 当前游戏 ID,避免同一小号 ID 在不同游戏下语义不清。 |
| `account` | query | 是 | 当前游戏小号 ID。 |
| `X-M5755-Token` | header | 是 | 游戏小号登录接口签发的登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `valid` | **必返字段**:当前游戏小号登录态是否有效。 |
| `account` | 当前游戏小号 ID。 |
| `uid` | 如返回,语义也必须是当前游戏小号 ID,不能是 5755 账户 ID。 |
| `platformAccountId` | 如需返回 5755 账户 ID,使用该字段。 |
| `gameId` | 游戏 ID。 |
| `displayName` | 展示名。 |

业务规则:

- HTTP 成功和 `success=true` 只表示接口调用成功,不等于登录态放行。**成功响应必须包含 `data.valid`**,SDK 必须继续检查:
  - `data.valid=false`(`reason=subaccount_invalid`)→ 判定小号登录态无效,进入小号选择页;
  - 主账户级失效(踢号、账号停用)→ `data.valid=false` 且 `reason=platform_account_invalid`,回 5755 账户登录窗;
  - `data.account` 或 `data.uid` 与当前游戏小号 ID 不一致 → 判定登录态归属不一致,按无效处理;
  - `success=false` → 接口/平台失败,阻断但不误判失效。
- 上述任一无效情况下,SDK 必须阻断后续角色上报和支付前置链路,不再调用 `PUT /roles` 或 `POST /orders`。
- v2 不保留任何旧式万能令牌兼容路径;只认 2.7.1 签发的小号令牌。

### 2.8 角色上报 `PUT /api/sdk/v2/roles`

用途:把角色数据归属到当前游戏小号,按归属键 upsert(同键再次上报即更新)。字段命名与 SDK 公开 `RoleInfo` 一致。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `account` | body | 是 | 当前游戏小号 ID。 |
| `token` | body | 是 | 游戏小号登录令牌。 |
| `serverId` | body | 是 | 区服 ID;无区服传 `-1`。 |
| `serverName` | body | 是 | 服务器名称;无服务器传 `-1`。 |
| `roleId` | body | 是 | 游戏内唯一角色 ID;**不得为空或 `-1`,服务端校验并拒绝**(`reason=param_invalid`)。游戏无角色区分时不应调用角色上报,不得伪造角色 ID。 |
| `roleName` | body | 是 | 角色名称。 |
| `roleLevel` | body | 是 | 角色等级。 |
| `roleCE` | body | 是 | 角色战力;无战力传 `-1`。 |
| `roleStage` | body | 是 | 角色关卡;无关卡传 `-1`。 |
| `roleRechargeAmount` | body | 是 | 角色总充值;**`"-1"` 或保留 2 位小数的金额字符串**(如 `328.00`),服务端校验口径与 05 §1.2 一致,`"-1"` 必须被接受。 |
| `roleGuild` | body | 是 | 公会 ID;无公会传 `-1`。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `reported` | 上报结果,成功返回 `true`;不返回完整角色对象,角色详情留平台侧诊断。 |

业务规则:

- upsert 幂等键为 `gameId + account + serverId + roleId`;同键重复上报按更新处理,不报错。
- 服务端只读取本节规范字段,**不读取历史别名**(`totalRecharge`、`combatPower`、`progress`、`guildName` 等)。
- 小号令牌无效返回 `reason=subaccount_invalid`,SDK 按 03 §3 进入小号选择路径,不再继续上报或支付。

### 2.9 订单 `/api/sdk/v2/orders`

#### 2.9.1 支付创建 `POST /api/sdk/v2/orders`

用途:创建支付订单并交接客户端支付状态。订单归属当前游戏小号与当前角色字段。字段命名与 SDK 公开 `Order` 一致。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `gameId` | body | 是 | 当前游戏 ID。 |
| `account` | body | 是 | 当前游戏小号 ID。 |
| `token` | body | 是 | 游戏小号登录令牌,用于校验支付归属。 |
| `cpOrderId` | body | 是 | 游戏订单号;非空,**长度不超过 128,服务端强制校验**。 |
| `amount` | body | 是 | 金额,单位元,字符串金额;**必须大于 0,服务端强制校验**(`0`、`0.00`、负数均拒绝)。 |
| `commodity` | body | 是 | 商品名称。 |
| `serverId` | body | 是 | 支付区服 ID。 |
| `serverName` | body | 是 | 支付服务器名称。 |
| `roleId` | body | 是 | 支付角色 ID。 |
| `roleName` | body | 是 | 支付角色名称。 |
| `roleLevel` | body | 是 | 支付角色等级。 |

(`initialized` 为后端内部调试兼容字段,SDK 不发送;显式为 `false` 时后端返回 code=6。)

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `platformOrderId` | 平台订单号。 |
| `orderId` | 兼容平台订单号,可与 `platformOrderId` 一致。 |
| `paymentUrl` | H5 支付台入口;SDK 必须解析并校验该入口。生产环境必须是真实支付台地址,不得下发 mock 收银台。 |
| `account` | 当前游戏小号 ID。 |
| `cpOrderId` | 游戏订单号,与请求一致。 |
| `amount` | 与请求金额一致。 |
| `commodity` | 与请求商品一致。 |
| `serverId` / `serverName` | 与请求区服一致。 |

业务规则:

- **不读取任何历史别名字段**(`money`、`goods`、`server` 等);金额、商品、区服字段固定为 `amount/commodity/serverId/serverName`,别名出现时按未知字段忽略,缺规范字段即失败。
- 金额、商品、区服、角色、CP 订单号等支付归属字段缺失或非法时,**创建必须失败**,`reason=order_invalid`;禁止服务端默认值补齐(对齐 05 §2.3 禁止演示订单兜底)。
- **服务端复核防沉迷支付门禁**:创建订单时服务端按当前 5755 账户实名/年龄状态复核支付门禁,命中即拒绝,`reason=anti_addiction_payment_blocked`,仅失败本次支付,不触发账号变化。客户端门禁照旧(体验层前置提示),服务端复核是防绕过的最后防线。
- 小号令牌无效返回 `reason=subaccount_invalid`;支付归属(令牌-小号-游戏)不一致必须失败。
- 客户端支付回调只作为 UI 或处理中提示;游戏内物品发放必须以游戏服务端收到并校验通过的充值回调(服务端)为准(见第 4 节)。

#### 2.9.2 订单查询 `GET /api/sdk/v2/orders/{orderId}`

用途:按需查询支付订单状态的排障/轮询能力,不阻断最小支付创建链路,不替代充值回调(服务端)发货。

请求字段:

| 字段 | 位置 | 必填 | 说明 |
| --- | --- | --- | --- |
| `orderId` | path | 是 | 平台订单号。 |
| `X-M5755-Token` | header | 是 | 当前游戏小号登录令牌,**本端点必须鉴权**。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `paymentStatus` | 支付状态,如 `待支付` / `已支付`。 |
| `callbackStatus` | 充值回调(服务端)状态,仅用于诊断。 |

业务规则:

- **仅订单归属小号可查**:服务端按小号令牌解析归属,订单不存在或不归属该小号统一返回失败(`reason=order_invalid`),不区分"不存在"与"无权查看",避免订单号探测。
- **响应只返回 `paymentStatus/callbackStatus` 两个字段**,不返回金额、角色等完整订单对象,收敛信息暴露面。
- 查询结果不影响客户端支付回调的提示语义,更不作为发货依据。

## 3. dev 控制面 `/internal/dev-control/*`(dev-only)

dev 控制面为 03 阻断/回退矩阵与 05 异常路径提供服务端可控的异常注入能力,服务《08》异常路径回归;**不属于 SDK 契约承诺**,SDK 生产代码不感知这些端点。

| 端点 | 方法 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `/internal/dev-control/maintenance` | POST | 设置维护门禁,驱动 `GET /config` 的 `maintenance` | `gameId`、`enabled`、`message` |
| `/internal/dev-control/anti-addiction` | POST | 注入账户级防沉迷门禁,覆盖 `/real-name` 返回值 | `gameId`、`platformAccountId`、`entryBlocked`、`paymentBlocked` |
| `/internal/dev-control/kick` | POST | 踢号:吊销主账户会话,使会话类端点返回 `platform_account_invalid` | `gameId`、`platformAccountId` |
| `/internal/dev-control/invalidate-subaccount` | POST | 小号失效:吊销小号令牌/停用小号,使小号会话端点返回 `subaccount_invalid` | `gameId`、`account` |
| `/internal/dev-control/fault` | POST | 故障注入:对指定端点注入异常响应 | `gameId`、`endpoint`、`type`(`delay` / `http500` / `malformed`)、`delayMs`(delay 用)、`times`(**默认 1,消耗完自动失效**) |
| `/internal/dev-control/complete-payment` | POST | 推进订单到"已支付"并触发一次充值回调投递(**收编原 `POST /api/mock/pay/{orderId}`,该原端点在 v2 不复存在**) | `gameId`、`orderId`、`mode`(回调投递模式:成功/失败/超时等) |
| `/internal/dev-control/reset` | POST | 重置作用域内全部注入态与测试数据 | `gameId` |
| `/internal/dev-control/state` | GET | 查询当前注入状态,供回归断言与排障 | `gameId`(query) |

统一约束:

- **全部按 `gameId` 作用域生效**,不同游戏的注入态互不影响;操作可观察(`state`)、可回滚(`reset`)。
- 响应统一为 `ApiResult` 信封。
- **三重生产防护**:
  1. 生产构建**不注册** `/internal/*` 路由(不是运行时开关关闭,而是路由不存在);
  2. 发布门禁探测 `/internal/*` 必须返回 404,否则不得发布;
  3. dev 环境调用 dev 控制面同样需要 dev 测试密钥签名(1.3 口径),无签名调用拒绝。
- 维护、踢号、小号失效、防沉迷注入后的 SDK 行为断言,以 03 §3 阻断/回退表与本契约 1.2.1 reason 表为准。

## 4. 充值回调(服务端,Payment Callback)

充值回调是 5755 平台服务端到游戏服务端的通知,**不是 Android SDK 客户端端点**,Android SDK 不实现客户端回调端点,AAR 不包含发货逻辑。

回调字段:

| 字段 | 说明 |
| --- | --- |
| `account` | 当前游戏小号 ID。 |
| `platformOrderId` | 平台订单号(SDK 业务口径)。 |
| `cpOrderId` | 游戏订单号(SDK 业务口径)。 |
| `amount` | 订单金额(SDK 业务口径)。 |
| `commodity` | 商品名称(SDK 业务口径)。 |
| `serverId` / `serverName` | 区服字段(SDK 业务口径)。 |
| `order_id` | 平台订单号(历史兼容)。 |
| `cp_order_id` | 游戏订单号(历史兼容)。 |
| `money` | 订单金额(历史兼容)。 |
| `pay_money` | 实付金额。 |
| `sign` | 按平台签名规则生成的签名。 |

游戏服务端成功响应:

```json
{
  "code": 200,
  "msg": "success"
}
```

责任划分:

- **平台服务端**:在支付完成后向游戏服务端发起回调;`account`、订单、金额与签名语义必须与支付创建一致;回调可能重复发送。
- **游戏服务端**:负责验签、金额校验、账号归属校验和幂等发放——重复回调必须幂等处理,金额或订单归属不一致必须失败。发货不依赖 Android 客户端状态。
- **Android SDK**:只负责创建订单、接收平台订单号和支付入口、展示客户端支付状态(仅 UI 提示)。

## 5. 运维面

| 端点 | 方法 | 说明 |
| --- | --- | --- |
| `/healthz` | GET | 部署健康检查,供编排/负载均衡探活。 |
| `/openapi.json` | GET | 机器契约自描述,供构建期做 SDK 网关与平台服务端两端契约一致性校验。 |

- 运维面端点**无签名、只读**,不暴露业务数据与敏感配置;不属于 SDK 业务契约,SDK 运行时不调用。

## 6. 环境矩阵与产物配置

平台环境与 host 来自 AAR 包内 `assets/m5755-sdk-platform.properties`,由 SDK 构建或发布配置固定;接入游戏不能通过运行时参数、Manifest、透传或 H5 把生产 AAR 切到 dev、mock 或 fixture。联调 AAR 和生产 AAR 使用相同业务契约。

| 环境 | `artifactType` | `platformEnv` | `baseHost` | 用途 |
| --- | --- | --- | --- | --- |
| 生产 | `production` | `prod` | `sdk.xingninghuyu.com` | 生产 AAR,达到上线标准后连接(新平台服务端生产部署)。 |
| 联调 | `integration` | `dev` | `sdk-dev.xingninghuyu.com` | **v2 开发与联调阶段的默认环境**:新版本实现、模拟器验收均对接新平台服务端的 dev 部署;切生产须经发布门禁。 |
| 本地验收 | `local` | `local` | `127.0.0.1`(本地等价服务,如 `127.0.0.1:4173`) | 仅构建期契约验收,不进入公开 AAR 交付。 |

dev 控制面在各环境的可用性:

- dev 与本地验收部署注册 `/internal/dev-control/*` 路由(第 3 节),并提供公开测试签名密钥与 mock 短信模式;
- 生产部署不注册 `/internal/*` 路由,发布门禁探测必须 404(第 3 节三重防护)。

`m5755-sdk-platform.properties` 字段:

| 字段 | 说明 |
| --- | --- |
| `artifactType` | 产物类型(同上表)。 |
| `platformEnv` | 平台环境(同上表)。 |
| `baseHost` | 平台接口域名;包内写入裸域名,SDK 运行时默认补 `https://`。 |
| `signatureConfigVersion` | 签名配置非密版本标识。 |
| `keyId` | 签名 key 非密标识。 |
| `version` | 产物版本。 |

产物校验绑定:

- 发布门禁对产物配置做绑定校验:公开 AAR 不得包含模拟网关、fixture、sample、test、mock、demo 与本地成功文案;生产产物不得指向 dev/local host。
- 生产目标禁止运行会创建测试账号、小号、订单和调试支付的业务 smoke;生产只执行外部可用性、OpenAPI 机器契约和受控外部验收。
