# 游戏服务端面鉴权统一 HMAC-SHA256:引入 serverKey + 充值回调签名 MD5 改 HMAC(取代 #73)

## 背景

为给**游戏接入方的服务端**写对接文档 / 机器可读契约(经 grill 从 U10 旧平台的开发者文档对照而来),盘点本项目对外鉴权,发现两件事:

1. **登录态校验无鉴权口径(设计 gap)**:`integration-guide §3.3` 让游戏服务端"用 account+token 做登录态校验",对应接口是 `GET /api/sdk/v2/subaccount-sessions`(04 §2.7.2,自标「= U10 `oauth/check`」)。但该接口在 **SDK 网关面**、按 `04 §1.3` 必须 HMAC 签名,而 §1.3 明文"**密钥由 SDK 构建固定**、不能由接入游戏切换"——密钥焊在 AAR 里。**游戏服务端是独立主体、没有也不该有这把密钥**,04 从未定义它拿什么鉴权调这个端点。

2. **对外签名三套不一致**:SDK 网关面 = HMAC-SHA256(§1.3);充值回调 = MD5(#73 / 04 §4);uc 面 = Bearer(ADR-0010)。游戏服务端一旦同时碰「登录态校验」+「充值回调」,就要学两套签名(若登录态校验补 HMAC、回调仍 MD5)。

本 ADR 经 grill(2026-06-15)产出,定游戏服务端面的鉴权口径,并据此**取代 #73 的 MD5 决定**。

## 决定

1. **引入游戏服务端独立 `serverKey`**:每游戏发一对 `serverKeyId + serverSecret`(平台后台发放,纳入每游戏配置)。平台按 `keyId` 区分调用主体(SDK keyId vs serverKeyId),用对应密钥验签。游戏服务端**不碰** AAR 焊死的 SDK 密钥。

2. **游戏服务端面统一 HMAC-SHA256**,一套 `serverKey` 两个用途:
   - **登录态校验**(游戏服务端 → 平台,`GET /api/sdk/v2/subaccount-sessions`):游戏服务端用 `serverKey` 按 §1.3 同样的 HMAC-SHA256 + 时间戳防重放签名。
   - **充值回调**(平台 → 游戏服务端):平台用同一 `serverSecret` 以 **HMAC-SHA256** 签,游戏服务端验——**取代 #73 的 MD5**。

3. **充值回调签名 MD5 → HMAC-SHA256(取代 #73)**。前提:v2 是全新平台、首款游戏尚在适配,接入方均**新写验签代码、无旧 MD5 包袱**(本场 grill 确认)。故不再为兼容旧惯例保 MD5。

4. **不卷入统一的**(合理分层,保持不动):`uc` 面 Bearer(玩家不持签名密钥、用会话令牌 platformToken,ADR-0010);渠道入站回调原生签名(微信 / 支付宝,外部决定、不可改)。统一只针对「5755 ↔ 接入方」的签名。

5. **对外契约形态**:游戏服务端面(登录态校验 + 充值回调 webhook)做**服务端面 OpenAPI**(机器可读契约,**不搭 Swagger 站**);客户端接入面仍走 `integration-guide`(Java API `com.m5755.operate.api.*`,OpenAPI 描述不了)。**04 整体是 SDK↔平台内部网关契约,不对外做 Swagger**——本项目有 AAR,游戏客户端调 Java 方法、不直接调 04 的 HTTP,与 U10(无 AAR、开发者直调 HTTP)不同。

## 为什么

- **游戏服务端是独立主体**,该有独立 `serverKey`;复用 SDK 焊死的密钥违 §1.3「不透传」且把 AAR 密钥泄给服务端。
- **统一 HMAC**:游戏服务端只学一套签名(HMAC-SHA256),登录态校验与充值回调一致;全平台对外签名收敛为「HMAC(SDK + 服务端)+ uc Bearer(玩家)+ 渠道原生(入站)」,不再三套并存。
- **MD5 弱于 HMAC-SHA256**:裸 MD5 字典序拼接 + 密钥的构造,抗碰撞 / 抗长度扩展弱于 HMAC;统一到 HMAC 是安全提升。
- **取代 #73 代价可接受**:#73 保 MD5 唯一理由是兼容旧游戏服务端 / CP 的 MD5 验签惯例;v2 无旧接入,理由不成立。

## 考虑过的其他选项

- **(a) 保 #73 MD5,密钥统一算法各用**(登录态校验 HMAC + 回调 MD5 共用 serverSecret)— 否决:游戏服务端仍要学两套算法,没达成"统一";且一密钥两算法不洁。
- **(b) 游戏服务端面统一 MD5**(登录态校验也 MD5)— 否决:MD5 弱于 HMAC,且与 SDK 网关面不齐。
- **(c) 独立游戏服务端面 `/api/server/*`**(新面 + 独立中间件,与网关面 / uc 面三面并列)— 未选:最清晰但最大改动,现无真实服务端对接方、过早;`serverKey` 复用 §1.3 + 现有 `subaccount-sessions` 端点已够。
- **(d) `serverKey` 复用 SDK 的 HMAC 密钥** — 否决:SDK 密钥焊 AAR、不该给服务端(§1.3 不透传)。
- **(e) serverKey 独立 + 服务端面统一 HMAC + 取代 #73 MD5** — **选中**:游戏服务端一套密钥一套算法,全平台对外签名收敛。

## 后果

- **取代 #73**:充值回调签名口径由 MD5 改为 HMAC-SHA256;#73 的 MD5 落定(04 §4)被本 ADR 推翻。
- **改动面(拆为实现 issue)**:`04 §1.3 / §2.7.2`(serverKey 鉴权 + 游戏服务端调用口径)、`04 §4`(回调 HMAC)、`05`(回调签名同步)、`domain_m3.go callbackSign`(md5 → hmac-sha256)、`mock-gameserver`(#69,复刻验签同步)、`smoke-alipay`(#69,回调验签处)、`integration-guide §5`(回调验签算法 + §3.3 登录态校验接口 + serverKey)、相关单测。
- **新增**:游戏服务端面 OpenAPI(`subaccount-sessions` GET + 充值回调 webhook,含 `serverKey` HMAC security)。
- **serverKey 发放**:纳入每游戏配置,与生产密钥注入一并(GA 前置)。
- **实现细节留拆出的 issue 定**:HMAC 回调的 canonical 构造(字典序拼接 vs body 签)、防重放窗口、serverKeyId 命名空间与 SDK keyId 的区分、serverKey 轮换。
- `integration-guide` 客户端面与服务端面**分层**:客户端 = Java API(本指南),服务端 = OpenAPI 契约 + 本指南服务端章节。

## 更新(2026-06-15 grill「检查项目」):4 个延后决定收口 + 充值回调体定形

经第二轮 grill,上「后果」段列为「留拆出的 issue 定」的实现细节逐条定稿,并顺带把充值回调体收敛去冗余(遵本仓「不搬运」)。

**4 个延后决定的最终口径**

- **canonical 构造**:回调 `sign` = `HMAC-SHA256(serverSecret, 除 sign 外全字段按键字典序逐对 "k=v&" 含末对)` hex 小写(已落地 `domain_m3.go callbackSign` + 04 §4)。
- **serverKeyId 命名空间**:与 SDK keyId 同 `signing_keys` 表、靠 `principal` 列('sdk'/'server')区分;`principalScope()` 把 serverKey 限死只能调 `GET subaccount-sessions`,越权 403 `principal_not_allowed`(已落地 #84/#86 + 04 §1.3)。
- **防重放窗口**:充值回调**不设**时间戳/窗口——`RedeliverPendingCallbacks` 自愈巡检会重投与首发逐字节相同的陈旧但有效回调,加窗口会与自愈互斥。**游戏服务端以 `orderId` 幂等去重**(= 重复投递与重放的唯一防线);`cpOrderId`/`amount`/`account` 仅作一致性交叉校验。入站登录态校验仍走 §1.3 ±300s 窗口——出入站不对称是自愈设计的必然,非疏漏。
- **serverKey 轮换**:v2 默认每游戏**单把 active serverKey**;**回调体新增非密 `serverKeyId` 字段**(进签名串),游戏据它选密钥验签 → 把「换密钥」从契约变更降为配置操作、与入站 header keyId 的轮换能力对称。

**充值回调体定形**

- 砍下划线别名 `order_id` / `cp_order_id` / `money`(本场 grill 确认**无旧接入吃这些键**;「无旧包袱」从签名算法层延伸到字段层)。
- `platformOrderId` → **`orderId`**(02 已立词:裸词本归平台,`platform*` 前缀只留给被小号占用的 `account` / `token`)。
- `pay_money` → **`payAmount`**,保留作前向兼容缝(ADR-0012;v2 恒等 `amount`)。
- 新增 `serverKeyId`(见上)。
- 最终回调体:`account · orderId · cpOrderId · amount · payAmount · commodity · serverId · serverName · serverKeyId · sign`。

**待落地改动面**:04 §4 / §2.7.2、05、`server-facing-openapi.yaml`、`integration-guide §5`、`domain_m3.go`(回调体 + per-game serverKey 选密钥签)、migrations(`orders` / `payment_notifications` 列 `platform_order_id` → `order_id`,新增 `0011`)、`mock-gameserver` / `smoke-alipay`(验签复刻 + `orderId` 幂等 + `serverKeyId` 选密钥)、Android `Results.java` / `HttpPlatformGateway.java` / `ColdStartController.java`、相关单测。02 术语已即时回写(`orderId` / `cpOrderId` / `amount` / `payAmount` / `serverKey`)。
