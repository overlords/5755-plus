# 04 平台网关 API(JSON 契约)

本文档定义 5755 Android SDK 内部平台网关与平台后端之间的 HTTP JSON 契约。这些接口由 SDK 内部网关发起,不向接入游戏暴露;接入方只使用 `com.m5755.operate.api.*` 公开白名单 API。

字段口径(全文适用):

- `account` 固定表示当前**游戏小号** ID,任何接口都不得用它表示 5755 账户(平台主账户)ID。
- 5755 账户使用 `platformAccountId` / `platformToken`(平台主账户登录令牌)。
- 游戏小号登录令牌使用 `token`,仅由小号登录接口签发。
- **充值回调(服务端)**指平台服务端到游戏服务端的发货通知;**客户端支付回调**仅作为 UI 或处理中提示,不作为物品发放依据。

## 1. 统一约定

### 1.1 路径与传输

- 所有 SDK-facing 接口使用 `/api/sdk/v1/...` 路径;以 `POST` 为主,订单查询为 `GET`。
- 传输使用 HTTPS。SDK 对包内配置的裸域名运行时默认补 `https://`。
- 请求体使用 JSON,`Content-Type: application/json; charset=UTF-8`,并携带 `Accept: application/json`。
- 旧协议(`/sdk/...` 路径、form-urlencoded 请求、properties 文本响应)、`login/complete`、`subaccounts/create-first` 均不在本契约内,不得回流。

### 1.2 统一响应 ApiResult

所有响应统一为 JSON `ApiResult`:

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `success` | boolean | 业务是否成功。 |
| `code` | int | 业务码;SDK 失败时归一到现有公开 `OperateCode`,不新增公开错误码。 |
| `message` | string | 业务信息;保留后端业务原因。 |
| `data` | object | 成功接口按能力返回的数据体。 |

失败处理规则:

- 后端业务失败可使用非 2xx HTTP 状态,但响应体仍必须保持 `ApiResult` 形态;SDK 必须读取失败响应体中的 `code/message`,不得因 HTTP 状态丢失业务原因。
- HTTP 成功且 `success=true` 仅表示接口调用成功,不等于业务放行(详见各端点业务规则,如 `oauth/check` 的 `valid`)。
- JSON 解析必须忽略后端新增的未知字段;缺少 SDK 必需字段时必须明确失败并输出可排查诊断。
- 响应 JSON 不可解析时按失败处理。

### 1.3 请求头与签名标识

每个请求携带产物标识请求头,取值来自 AAR 包内平台配置(见第 6 节):

| 请求头 | 说明 |
| --- | --- |
| `X-M5755-Artifact-Type` | 产物类型:`integration` / `production` / 构建验收用 `local`。 |
| `X-M5755-Platform-Env` | 平台环境:`dev` / `prod` / 构建验收用 `local`。 |
| `X-M5755-Key-Id` | 签名 key 的非密标识。 |

签名与密钥约束:

- `keyId`、签名配置版本(`signatureConfigVersion`)等只允许出现非密标识;诊断与日志禁止输出签名密钥、签名原文等完整签名材料。
- 环境、证书、签名配置、keyId、日志级别和平台 host 只能由 SDK 构建或发布配置固定,不能由接入游戏通过初始化参数、Manifest、H5、透传或运行时开关切换。

### 1.4 超时与重试

| 项 | 取值 |
| --- | --- |
| 连接超时 | 5 秒 |
| 读取超时 | 5 秒 |
| 单次请求总等待 | 9 秒,超出按超时失败(`OperateCode.TIMEOUT`) |
| 自动重试 | 网关不做自动重试;失败归一到 SDK 回调,由业务流程决定后续动作 |

### 1.5 公共请求字段

| 字段 | 说明 |
| --- | --- |
| `gameId` | 当前游戏 ID,5755 平台分配。 |
| `platformAccountId` | 5755 账户 ID;实名和小号类接口必传。 |
| `platformToken` | 5755 账户登录令牌;实名和小号类接口必传。 |
| `account` | 当前游戏小号 ID,按接口需要传入。 |
| `token` | 当前游戏小号登录令牌,按接口需要传入。 |
| `channelId` | SDK 解析后的渠道标识符,用于 5755 账户渠道归因;不允许游戏侧透传覆盖。 |
| `channelSource` | 渠道字段来源,如 Manifest、APK Signing Block 或默认值。 |

### 1.6 诊断与敏感信息

- SDK 诊断可记录:`requestId`、`baseHost`、`platformEnv`、`artifactType`、`configVersion`、`signatureConfigVersion`、`keyId` 等非密字段,用于与平台日志对齐。
- 诊断、日志与验收报告禁止记录:完整验证码(含 mock `devCode`)、密码、完整 `credential`、明文实名资料、签名密钥与完整签名材料、完整 token。登录账号只保留掩码。

## 2. 端点契约

### 2.1 初始化 `POST /api/sdk/v1/init`

用途:拉取初始化配置、维护门禁、防沉迷门禁、协议版本与诊断字段。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `sdkVersion` | 是 | SDK 版本。 |
| `packageName` | 是 | 宿主包名。 |
| `channelId` | 是 | 渠道标识符。 |
| `channelSource` | 是 | 渠道来源。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `gameId` | 游戏 ID。 |
| `gameName` | 游戏名称。 |
| `maintenance` | 维护门禁对象:`enabled`(是否维护中)、`message`(维护提示)。 |
| `antiAddictionEntryBlocked` | 防沉迷进入游戏门禁。 |
| `antiAddictionPaymentBlocked` | 防沉迷支付门禁。 |
| `protocolVersion` | 协议告知版本。 |
| `accountNickname` | 5755 账户昵称,仅用于 SDK UI 展示。 |
| `requestId` | 排障诊断 ID。 |
| `configVersion` | 平台配置版本。 |
| `sdkLatestVersion` / `sdkMinVersion` / `updateRequired` | SDK 版本信息与是否强制更新。 |
| `loginDomain` / `paymentDomain` | 登录域、支付域。 |

业务规则:

- 维护门禁、防沉迷门禁、协议版本、`requestId`、`configVersion` 必须来自真实平台响应,不允许本地默认成功配置放行。
- 真实初始化失败时必须阻断进入流程,失败状态可观察,中间态不得伪装成业务成功。
- `passthrough` 透传字段已废弃,生产 SDK 不发送;不允许用透传控制环境、fixture 或业务链路。

### 2.2 短信验证码请求 `POST /api/sdk/v1/sms`

用途:玩家在验证码登录页点击"发送验证码"时,请求平台向登录页输入的手机号发送一次验证码。该接口只用于取得本次 5755 账户登录凭据,不表示登录成功、注册入口或设备验证;发生在验证码登录提交之前。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `loginAccount` | 是 | 登录页输入的手机号(登录标识)。不得使用游戏小号 `account` 字段。 |

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

### 2.3 平台账户登录 `POST /api/sdk/v1/login`

用途:5755 账户登录的唯一入口。SDK 不提供单独注册通道;验证码登录和密码登录都是登录方式,账号是否存在、是否需要创建 5755 账户由服务端识别并处理。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `loginMethod` | 是 | 登录方式:`sms` 或 `password`。 |
| `loginAccount` | 是 | 玩家输入的手机号或账号。不得使用游戏小号 `account` 字段。 |
| `credential` | 是 | 验证码或密码等登录凭据。SDK 不得在日志、诊断或错误信息中输出完整 `credential`。 |
| `channelId` | 是 | SDK 解析后的渠道标识符。 |
| `channelSource` | 是 | 渠道来源。 |

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
- 首个游戏小号属于服务端行为:新 5755 账户首次进入当前游戏时,服务端在本接口内保障首个真实游戏小号存在(`isDefault=false`);SDK 不调用任何 `create-first` 类接口。
- `channelId/channelSource` 用于 5755 账户新用户渠道归因;老用户后续登录不得覆盖既有归因。
- 登录失败时 SDK 必须能向调用方呈现后端 `code/message`,以区分凭据错误、账号无效、网络失败和平台不可用。

### 2.4 账户有效检查 `POST /api/sdk/v1/account/validate`

用途:SDK 自动登录或登录后检查 5755 账户是否仍有效。本地缓存的 5755 登录态只能触发自动登录或本检查,不能替代真实检查。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `platformAccountId` | 是 | 5755 账户 ID。 |
| `platformToken` | 是 | 5755 账户登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `valid` | 5755 账户是否有效。 |
| `platformAccountId` | 5755 账户 ID。 |
| `displayName` | 展示名。 |

业务规则:

- 检查失败或账户失效时,SDK 必须清理当前游戏小号态并返回 5755 账户登录窗口,不进入小号选择;不能用本地 token 放行。

### 2.5 实名认证检查 `POST /api/sdk/v1/real-name/verify`

用途:检查 5755 账户实名状态。实名认证归属于 5755 账户,不归属于游戏小号。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `platformAccountId` | 是 | 5755 账户 ID。 |
| `platformToken` | 是 | 5755 账户登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `verified` | 是否已实名。 |
| `adult` | 是否成年。 |
| `antiAddictionEntryBlocked` | 防沉迷进入游戏门禁。 |
| `antiAddictionPaymentBlocked` | 防沉迷支付门禁。 |

业务规则:

- 未实名时进入实名提交流程,实名通过后继续小号流程。
- SDK 必须消费本接口返回的防沉迷门禁(写入运行态),不只依赖初始化配置:`antiAddictionEntryBlocked=true` 阻断进入游戏;`antiAddictionPaymentBlocked=true` 阻断支付,但保持当前游戏小号登录态,不触发登出或账号变化通知。
- 响应只返回脱敏实名状态,不回显明文实名资料。

### 2.6 实名认证提交 `POST /api/sdk/v1/real-name/submit`

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `platformAccountId` | 是 | 5755 账户 ID。 |
| `platformToken` | 是 | 5755 账户登录令牌。 |
| `realName` | 是 | 真实姓名。 |
| `idNumber` | 是 | 身份证号。 |

响应 `data` 字段:与 2.5 一致(`verified`、`adult`、`antiAddictionEntryBlocked`、`antiAddictionPaymentBlocked`),只返回脱敏实名状态,不回显明文实名资料。

业务规则:

- SDK 诊断禁止记录明文实名资料。
- 提交返回的防沉迷门禁与 2.5 同口径,SDK 必须消费。

### 2.7 游戏小号列表 `POST /api/sdk/v1/subaccounts/list`

用途:返回当前 5755 账户在当前游戏下的真实游戏小号列表,驱动小号选择页。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `platformAccountId` | 是 | 5755 账户 ID。 |
| `platformToken` | 是 | 5755 账户登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `defaultAccount` | 默认游戏小号 ID;未设置时为空。 |
| `subaccounts[]` | 小号数组,每项至少包含 `account`(游戏小号 ID)、`displayName`(展示名)、`isDefault`(是否默认)。 |

业务规则:

- 小号选择页只展示本接口返回的真实小号;演示小号、本地小号、本地默认列表不得进入成功路径。
- 首个小号由平台在 `/api/sdk/v1/login` 内保障创建并通过本接口返回;**列表为空时 SDK 阻断登录并输出诊断**,不在客户端补建、不调用 `create-first`。
- 只有真实列表中存在默认小号时,SDK 才展示轻量自动进入提示;没有默认小号时进入完整小号选择页。上次游戏小号不作为自动进入依据。

### 2.8 添加游戏小号 `POST /api/sdk/v1/subaccounts/create`

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `platformAccountId` | 是 | 5755 账户 ID。 |
| `platformToken` | 是 | 5755 账户登录令牌。 |

响应 `data` 字段(新增小号):

| 字段 | 说明 |
| --- | --- |
| `account` | 新增游戏小号 ID。 |
| `label` / `displayName` | 新增小号展示名。 |
| `defaultAccount` / `isDefault` | 必须为 `false`:新增小号不自动成为默认小号。 |

业务规则:

- 新增小号不自动成为默认小号;默认身份必须由玩家显式设置(见 2.9)。请求中即使后端兼容 `isDefault` 字段,SDK 也不得用它自动设置默认。
- 已知失败场景:当前游戏下小号数量已达 10 个;5755 账户失效;网络或平台不可用。

### 2.9 设置默认游戏小号 `POST /api/sdk/v1/subaccounts/set-default`

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `platformAccountId` | 是 | 5755 账户 ID。 |
| `platformToken` | 是 | 5755 账户登录令牌。 |
| `account` | 是 | 要设为默认的当前游戏小号 ID。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `account` | 游戏小号 ID。 |
| `defaultAccount` | `true`,设置结果。 |

业务规则:

- 只有玩家点击小号行上的默认标签才调用本接口;点击小号行进入游戏不等于设置默认。

### 2.10 游戏小号登录 `POST /api/sdk/v1/subaccounts/login`

用途:使用玩家选择或默认的游戏小号换取当前游戏小号登录令牌。**SDK 最终返回给游戏的 `account/token` 只来自本接口**。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `platformAccountId` | 是 | 5755 账户 ID。 |
| `platformToken` | 是 | 5755 账户登录令牌。 |
| `account` | 是 | 玩家选择或默认的游戏小号 ID。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `account` | 当前游戏小号 ID。 |
| `token` | 游戏小号登录令牌。 |
| `expiresAt` | 有效期。 |

业务规则:

- 本接口返回的 `account/token` 是游戏服务端登录态校验、角色上报和支付归属的唯一凭据。
- 本地默认游戏小号只能触发自动进入提示的前置流程,不能替代真实小号列表或本接口登录。
- 默认小号登录失败且后端明确返回"游戏小号无效"时,SDK 必须进入小号选择路线,不能用本地默认小号或当前小号继续进入游戏。

### 2.11 登录态校验 `POST /api/sdk/v1/oauth/check`

用途:校验当前游戏小号登录态(不是 5755 账户登录态);游戏服务端与 SDK 诊断使用同一小号令牌语义。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID,避免同一小号 ID 在不同游戏下语义不清。 |
| `account` | 是 | 当前游戏小号 ID。 |
| `token` | 是 | 游戏小号登录接口签发的登录令牌。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `valid` | 当前游戏小号登录态是否有效。 |
| `account` | 当前游戏小号 ID。 |
| `uid` | 如返回,语义也必须是当前游戏小号 ID,不能是 5755 账户 ID。 |
| `platformAccountId` | 如需返回 5755 账户 ID,使用该字段。 |
| `gameId` | 游戏 ID。 |
| `displayName` | 展示名。 |

业务规则:

- HTTP 成功和 `success=true` 只表示接口调用成功,不等于登录态放行。SDK 必须继续检查:
  - `data.valid=false` → 判定登录态无效;
  - `data.account` 或 `data.uid` 与当前游戏小号 ID 不一致 → 判定登录态归属不一致。
- 上述任一情况下,SDK 必须阻断后续角色上报和支付前置链路,不再调用 `role/report` 或 `payment/orders`。

### 2.12 角色上报 `POST /api/sdk/v1/role/report`

用途:把角色数据归属到当前游戏小号。字段命名与 SDK 公开 `RoleInfo` 一致。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `account` | 是 | 当前游戏小号 ID。 |
| `token` | 是 | 游戏小号登录令牌。 |
| `serverId` | 是 | 区服 ID;无区服传 `-1`。 |
| `serverName` | 是 | 服务器名称;无服务器传 `-1`。 |
| `roleId` | 是 | 游戏内唯一角色 ID;不得为空或 `-1`。游戏无角色区分时不应调用角色上报,不得伪造角色 ID。 |
| `roleName` | 是 | 角色名称。 |
| `roleLevel` | 是 | 角色等级。 |
| `roleCE` | 是 | 角色战力;无战力传 `-1`。 |
| `roleStage` | 是 | 角色关卡;无关卡传 `-1`。 |
| `roleRechargeAmount` | 是 | 角色总充值;`-1` 或保留 2 位小数的金额(如 `328.00`)。 |
| `roleGuild` | 是 | 公会 ID;无公会传 `-1`。 |

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `reported` | 上报结果。 |

### 2.13 支付创建 `POST /api/sdk/v1/payment/orders`

用途:创建支付订单并交接客户端支付状态。订单归属当前游戏小号与当前角色字段。字段命名与 SDK 公开 `Order` 一致。

请求字段:

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `gameId` | 是 | 当前游戏 ID。 |
| `account` | 是 | 当前游戏小号 ID。 |
| `token` | 是 | 游戏小号登录令牌,用于校验支付归属。 |
| `cpOrderId` | 是 | 游戏订单号;非空,长度不超过 128。 |
| `amount` | 是 | 金额,单位元,字符串金额;必须大于 0。 |
| `commodity` | 是 | 商品名称。 |
| `serverId` | 是 | 支付区服 ID。 |
| `serverName` | 是 | 支付服务器名称。 |
| `roleId` | 是 | 支付角色 ID。 |
| `roleName` | 是 | 支付角色名称。 |
| `roleLevel` | 是 | 支付角色等级。 |

(`initialized` 为后端内部调试兼容字段,SDK 不发送;显式为 `false` 时后端返回 code=6。)

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `platformOrderId` | 平台订单号。 |
| `orderId` | 兼容平台订单号,可与 `platformOrderId` 一致。 |
| `paymentUrl` | H5 支付台入口;SDK 必须解析并校验该入口。 |
| `account` | 当前游戏小号 ID。 |
| `cpOrderId` | 游戏订单号,与请求一致。 |
| `amount` | 与请求金额一致。 |
| `commodity` | 与请求商品一致。 |
| `serverId` / `serverName` | 与请求区服一致。 |

业务规则:

- 不使用历史兼容别名;金额、商品、区服字段固定为 `amount/commodity/serverId/serverName`。
- 客户端支付回调只作为 UI 或处理中提示;游戏内物品发放必须以游戏服务端收到并校验通过的充值回调(服务端)为准(见第 4 节)。

### 2.14 订单查询 `GET /api/sdk/v1/payment/orders/{orderId}`

用途:按需查询支付订单状态的排障/轮询能力,不阻断最小支付创建链路,不替代充值回调(服务端)发货。

请求:路径参数 `orderId`(平台订单号),无请求体。

响应 `data` 字段:

| 字段 | 说明 |
| --- | --- |
| `paymentStatus` | 支付状态,如 `待支付` / `已支付`。 |
| `callbackStatus` | 充值回调(服务端)状态,仅用于诊断。 |

业务规则:

- 查询结果不影响客户端支付回调的提示语义,更不作为发货依据。

## 3. 调试端点 `POST /api/mock/pay/{orderId}`

- 仅限 Demo App、预发布或测试环境用于推进调试订单到"已支付"状态。
- **不得由生产 AAR 调用**,不进入生产 SDK 支付完成链路,不能作为真实资金流转接口。

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

## 5. 环境矩阵与产物配置

平台环境与 host 来自 AAR 包内 `assets/m5755-sdk-platform.properties`,由 SDK 构建或发布配置固定;接入游戏不能通过运行时参数、Manifest、透传或 H5 把生产 AAR 切到 dev、mock 或 fixture。联调 AAR 和生产 AAR 使用相同业务契约。

| 环境 | `artifactType` | `platformEnv` | `baseHost` | 用途 |
| --- | --- | --- | --- | --- |
| 生产 | `production` | `prod` | `api.xingninghuyu.com` | 生产 AAR,达到上线标准后连接。 |
| 联调 | `integration` | `dev` | `dev.xingninghuyu.com` | **v2 开发与联调阶段的默认环境**:新版本实现、模拟器验收均对接 dev 后端(init/config 已实测可用);切生产须经发布门禁。 |
| 本地验收 | `local` | `local` | `127.0.0.1`(本地等价服务,如 `127.0.0.1:4173`) | 仅构建期契约验收,不进入公开 AAR 交付。 |

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
