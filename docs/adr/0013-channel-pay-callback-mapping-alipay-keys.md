# 渠道支付回调映射(异步=充值回调/同步=客户端回调)+ 支付宝接入(公钥模式 · dev 沙箱 · prod 生产)

## 背景

#60 接入微信/支付宝直连。grill(2026-06-14)确认:支付宝用「手机网站支付」(`alipay.trade.wap.pay`,即 H5 支付),持有**生产**密钥(公钥模式)、可建**沙箱**。本 ADR 钉住渠道接入的**回调映射**与**环境/密钥/构建结构**——以支付宝为准并对称推广到微信。

## 决定

**1. 渠道回调映射(核心,推广到所有渠道)**
渠道的**异步服务器通知(notify)= 充值回调的唯一触发源 = 物品发放依据**;渠道的**同步浏览器跳转(return)= 客户端支付回调 = 仅 UI「已交接」,绝不发货**。
- 支付宝 `notify_url` → `POST /pay/alinotify` → 支付宝公钥 RSA2 验签 + 金额/`trade_status`/订单存在/状态=待支付/幂等校验 → `CompletePayment` → `dispatchCallback`(充值回调 → 游戏服务端)→ 回 `success` 明文 ACK 止重推。
- 支付宝 `return_url` → 平台 sentinel `/pay/return?status=handed&orderId=<orderId>`(原 `platformOrderId`,ADR-0016 更新后改名;SDK 拦截为客户端支付回调「已交接」,见 ADR-0012 / #60 sentinel 契约)。`return_url` 只在付款成功(同步)触发;取消 = 玩家返回(无 return 命中)→ SDK 保守判「未完成」。
- 这是 `05 §3`「客户端支付回调 ≠ 充值回调」在渠道层的落地:**同步回调可被跳过/中断/伪造,不可作发货依据**。

**2. 支付宝密钥 = 公钥模式**
appid + 商户应用私钥(RSA2)+ 支付宝公钥,3 个 PEM;**非证书模式**(无 `app_cert_sn` 证书链)。出站下单用应用私钥签,验 notify/return 用支付宝公钥。优先 stdlib `crypto/rsa` + `crypto/sha256`。

**3. 环境与密钥注入(两端 fail-closed)**
网关 URL + 密钥按部署 **env 注入分流**:
- dev:支付宝沙箱(`openapi.alipaydev.com` + 沙箱凭据),注入 sdk-dev 部署;
- prod:生产(`openapi.alipay.com` + 真实商户),**真实商户私钥绝不上 dev 服务器**;
- 缺密钥 → 该路由 fail-closed(复用既有 `SIGNING_KEY` 注入模式;`bootstrap_prod.go` 校验)。

**4. 构建结构**
真收银台 + 预下单 + notify webhook 注册到**两个构建**(网关按 env 分流),**非 prod-only**(推翻 #60 原文「生产构建注册」);`/internal/dev-control/complete-payment` 保持 dev-only(注入回调的回归工具,与真支付正交);dev `/pay/{orderId}` mock 降为「未注入支付宝密钥时兜底」。映射:渠道 `out_trade_no`(支付宝渠道外部 wire 字段名,保持不动)= 平台 `orderId`(`P5755…`)。

## 考虑过的其他选项

- **用 `return_url`(同步)发货** — 否决:同步跳转可被玩家中断、不可达或伪造,会造成「跳回即发货」漏洞;发货必须只认服务器到服务器的 `notify`。
- **证书模式** — 未选:持有的是公钥模式 3 PEM,且证书模式(证书链 + 序列号)实现更重。
- **dev 走 mock、仅 prod 接真支付宝** — 未选:用户可建沙箱,dev 接沙箱即可端到端联调(真流程、无真钱)。

## 后果

联调:dev 走支付宝沙箱、prod 走小额真单(0.01 元 + 退款,对齐 `00-roadmap` GA 受控验收第 2 项)。`prod_exclusion_test.go` 仍守 dev-control 路由不入生产;但收银台/notify 不再是 dev 标记(本就该在生产)。**微信接入照本 ADR 的回调映射对称实现**(异步 notify=发货 / 同步 return=客户端回调 UI)。
