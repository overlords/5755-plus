# 支付宝沙箱端到端联调 Runbook

> 派生文档(随 `04`/`05`/ADR-0013 对齐,非权威 spec)。把支付宝**沙箱**整条支付链一次跑通:`下单 → 收银台 → wap → 沙箱付款 → 异步通知 → 平台验签反欺诈 → 充值回调送达游戏服务端 → 游戏侧验签通过`。依据 ADR-0013(dev 走沙箱、prod 走小额真单)。微信无沙箱,只能等真资质真单——支付宝这条链可在生产资质前先验。

## 充值回调签名口径(联调成败的关键)

**出站充值回调(平台→游戏服务端)签名 = MD5,不是 HMAC**,且**不复用** `internal/signature`(那是入站 SDK→平台的 HMAC-SHA256 + `X-M5755-Signature` 头)。真相源:`server/internal/domain/domain_m3.go:304-323 callbackSign`。

- 算法:`MD5(待签串)` → 十六进制小写。
- 待签串:取 body 中除 `sign` 外全部键,`sort.Strings` 字典序升序,逐对 `k=v&`(**每对都带 `&`,含最后一对**),末尾追加 `key=<secret>`(无尾随 `&`)→ 对整串 UTF-8 字节 MD5。
- 签名位置:JSON body 平级字段 `sign`(非 header)。
- 密钥:`CALLBACK_SECRET`,dev 默认 `m5755-dev-callback-secret-v1`(`bootstrap_dev.go:22`);prod 取 env、空则 fail-closed。
- 游戏侧确认:平台仅当 HTTP `200` **且** body `{"code":200,"msg":"success"}` 才判「已确认」,否则判失败并重投。

> **#59(OPEN)** 正评估是否把出站升级到 HMAC-SHA256 与入站对齐。升级时只需同步改 `callbackSign`(平台)与 `mock-gameserver` 的 `signCallback` 这一对函数——`cmd/mock-gameserver/main_test.go` 已用 `domain.VerifyCallbackSign` 把两者绑死,平台改了 mock 没跟会立即 test fail。

## mock 游戏服务端

`server/cmd/mock-gameserver/`:最小「游戏服务端」模拟器,接收平台充值回调 → 用与平台逐字节一致的方式验签 → 验过回 `{"code":200,"msg":"success"}` 并幂等记账、验败回 4xx → 每笔落结构化日志。**只用标准库**(不 import `internal/domain`,避免拖入 DB/Web 依赖),兼作 #59 出站签名参考实现 + GA 第 3 项游戏方对接样例。

```
go build ./cmd/mock-gameserver
./mock-gameserver -addr :18080 -path /callback -secret m5755-dev-callback-secret-v1
```

## env 注入模板(平台 dev 部署)

注入平台服务端进程环境后重启;`<...>` 为 HITL 持密项,从支付宝开放平台**沙箱**控制台取,绝不入码/入库。

```
# --- 支付宝沙箱渠道(任一缺失/非法 → 该渠道 nil + pay_channel_degraded 告警,fail-closed)---
ALIPAY_APP_ID=<沙箱应用 APPID>
ALIPAY_APP_PRIVATE_KEY_PEM=<沙箱应用 RSA2 私钥 PKCS8 PEM>
ALIPAY_PUBLIC_KEY_PEM=<支付宝沙箱公钥 PEM(是「支付宝公钥」,不是你的应用公钥)>
ALIPAY_GATEWAY=https://openapi.alipaydev.com/gateway.do   # 留空会默认正式网关,联调必须显式指沙箱

# --- 公网基址:notify_url/return_url/paymentUrl 同源推导;必须公网可达(沙箱要能直接 POST)---
PUBLIC_BASE_URL=https://sdk-dev.xingninghuyu.com           # notify_url = 此 + /pay/alinotify

# --- 出站回调签名密钥(mock 的 -secret 必须与此逐字一致;dev 默认即此串)---
CALLBACK_SECRET=m5755-dev-callback-secret-v1

DATABASE_URL=<平台 dev Postgres DSN>
```

## 凭据注入注意事项(已用真实沙箱凭据本地验证)

用一套真实支付宝沙箱凭据(appid + 应用私钥 + 支付宝公钥)本地验过 `paychannel.AlipaySigner`:`NewAlipaySigner` 加载 + `BuildWapPayURL` 出站签名 + 应用公钥自洽验签**全通过**(公私钥配对、PEM 兼容、签名算法正确)。注入时注意:

1. **裸 base64 必须先包成 PEM**:支付宝开放平台控制台导出的是**裸 base64**(无 `-----BEGIN-----` 头),而 `parseRSAPrivateKey`/`parseRSAPublicKey` 走 `pem.Decode`、**必须 PEM**。包装规则:
   - 应用私钥(PKCS#1,DER 以 `MIIEog…` 开头)→ `-----BEGIN RSA PRIVATE KEY-----` / `-----END RSA PRIVATE KEY-----`;
   - 支付宝公钥 / 应用公钥(PKIX,DER 以 `MIIBIj…` 开头)→ `-----BEGIN PUBLIC KEY-----` / `-----END PUBLIC KEY-----`;
   - base64 每 64 列换行(`fold -w64`)。
   不包 PEM 会 `NewAlipaySigner` 报「PEM 解码失败」→ 渠道 fail-closed、`pay_channels_assembled alipay=false`。
2. **沙箱网关地址务必核实可达**:`openapi.alipaydev.com`(ADR-0013 / 上文 env 模板写的老沙箱网关)在本地验证环境 **TLS 不可达**(`curl` exit 60 证书层失败,未到签名层)。注入前从支付宝开放平台沙箱控制台**核实当前沙箱网关域名**,并 `curl -sS <gateway> -o /dev/null -w '%{http_code}'` 确认 TLS 可达——老沙箱域名可能已变更 / 证书失效。
3. **`sign_type` 参与签名待真沙箱终验**:出站签名 canonical **含** `sign_type=RSA2`(支付宝请求签名口径),回调验签 `VerifyNotifySign` **剔除** `sign`+`sign_type`(V1 口径)。这两处只有真沙箱出网(下单)/ 真异步通知能终验;若沙箱回 `isv.invalid-signature`,优先排查 `sign_type` 规则。

## callback_url 配置(只能直连 DB)

平台**没有** HTTP 路由可改 `callback_url`(`store.SetCallbackURL` 仅 Go 测试夹具)。`dispatchCallback` 投递目标取自 `SELECT callback_url FROM games WHERE game_id=$1`(`store_m3.go`),该列 `NOT NULL DEFAULT ''`,空串 = 无回调地址、订单落「已支付/无回调地址」不投递。

```sql
UPDATE games SET callback_url='https://<mock 对平台可达的地址>/callback' WHERE game_id='m5755-demo';
-- 联调结束清理(避免后续测试误投):
-- UPDATE games SET callback_url='' WHERE game_id='m5755-demo';
```

`<地址>` 必须是**平台进程能直接 POST 到**的 mock 地址:同内网用内网地址,否则为 mock 起内网穿透(ngrok/frp)。末段须与 mock `-path` 一致(默认 `/callback`)。

## 联调步骤(逐步带断言;标 HITL 的需人工)

0. **前置编译**:`cd server && go build ./cmd/mock-gameserver && go build ./cmd/server` → 均 OK。
1. **公网穿透(HITL)**:两条入站必须公网可达——(a) 沙箱→平台 `/pay/alinotify`;(b) 平台→mock `/callback`。断言:`curl <公网域>/healthz` 均回 `{"ok":true}`。`callback_url` 写库的值须与 mock 实际可达地址一致。
2. **注入沙箱 env(HITL 持密)**:按上方模板注入平台并重启。断言:启动日志 `pay_channels_assembled alipay=true`(若 `alipay=false`/`degraded` 含 alipay → 密钥不全/非法,fail-closed,回查 env)。
3. **起 mock**:`./mock-gameserver -addr :18080 -path /callback -secret m5755-dev-callback-secret-v1`(secret 须与平台 `CALLBACK_SECRET` 逐字一致)。断言:日志 `mock_gameserver_start secretLen=28`;`curl localhost:18080/healthz` → `{"ok":true}`。
4. **配 callback_url(HITL,SQL)**:执行上方 `UPDATE`。断言:`SELECT` 回显刚写入的 mock 地址。**反向断言**:若跳过此步,后续订单会落「无回调地址」、`callback_skipped`、根本不投递——可据此排错。
5. **下单造待支付订单(HITL,带 HMAC 签名头)**:先登录拿小号 token,再 `POST https://sdk-dev.xingninghuyu.com/api/sdk/v2/orders`(body:`gameId=m5755-demo`/`account`/`token`/`amount`/`cpOrderId`/`commodity`/`serverId`/`serverName`/`roleId`/`roleName`/`roleLevel`)。**前置门禁**:`roleId` 非空、`amount∈(0,1e9]`、`cpOrderId` 非空且 ≤128;**且该账号须已过实名 + 防沉迷支付门禁**——未实名/被防沉迷挡的 dev 账户会被 `403`(`real_name_required`/`anti_addiction_pay_blocked`)挡在下单前,走不到支付。断言:`success=true`、`Data.platformOrderId` 形如 `P5755<纳秒>`、`Data.paymentUrl=…/pay/<platformOrderId>`。记下 `platformOrderId`。
6. **打开收银台选支付宝(HITL)**:浏览器/手机打开 `paymentUrl`。断言:渲染金额/商品 + 支付宝单选(占位页 = 渠道未注入,回 2);点「确认支付」→ `POST /pay/begin {orderId, method=alipay}` → 响应 `{success:true,data:{kind:'url',redirectUrl:<沙箱 wap URL>}}` → 外跳 `openapi.alipaydev.com`。
7. **沙箱钱包付款(HITL)**:用支付宝**沙箱版钱包** / 沙箱买家账号付款。断言:付款成功页;`return_url` 同步跳回 `/pay/return?status=handed`(**仅驱动 SDK UI,绝不据此发货**)。
8. **异步 notify 到达 + 验签反欺诈(观测平台日志)**:沙箱从公网 `POST …/pay/alinotify`。断言:平台回纯文本 `success`(止重推);日志显示 RSA2 验签过、`trade_status=TRADE_SUCCESS`、`total_amount` 折分与订单应收一致 → 幂等认领 → `CompletePayment`。收不到 notify ≈ `notify_url` 公网不可达或与 `PUBLIC_BASE_URL` 不一致(回 2 校验穿透域)。
9. **订单已支付 + 充值回调投递(平台侧)**:断言日志 `callback_attempt host=<mock 域> ok=true`、`callback_settled callbackStatus=已确认`。或走 SDK 网关面查单 `GET /api/sdk/v2/orders/<platformOrderId>`——注意:**`token` 必须放 HTTP 头 `X-M5755-Token`**(不是 query/body),`gameId`/`account` 是 query,`orderId` 是 path,整请求仍需带 HMAC 三件套(`X-M5755-Timestamp`/`Key-Id`/`Signature`),否则 401。断言:`paymentStatus=已支付`、`callbackStatus=已确认`。
10. **mock 收到回调且验签过(观测 mock 日志)**:断言 mock 日志 `callback_granted platformOrderId=<同上> amount=<同下单> signOK=true action=grant`——这证明「平台→游戏服务端验签通过并据此发货」闭环。
11. **幂等(可选)**:`POST /internal/dev-control/complete-payment {gameId,orderId,mode:超时}` 触发同笔重投,或等 `RedeliverPendingCallbacks` 巡检。断言:同一 `platformOrderId` **首次 `grant`、其余皆 `callback_idempotent_repeat action=ack_only`、无双发**(一次「超时」最多 6 次 POST:整体再投一轮 × 每轮最多 3 次重试,故可能多次 ack_only)。
12. **验签失败路径(可选,负向)**:向 mock 发 `sign` 被篡改的 payload。断言:mock 回 `401 {"code":401,"msg":"sign_invalid"}`、日志 `callback_sign_invalid signOK=false`;平台侧该次 `callback_attempt ok=false` → 投递失败 → 进重投巡检。

## 边界

- 沙箱联调验的是**平台侧 + 出站签名**全链;生产真单(GA 第 2 项)仍需真实商户密钥 + 小额真单对账。
- 充值回调是发货唯一依据;`return_url`/客户端回调只驱动 UI,绝不发货(ADR-0013 / `05 §3`)。
