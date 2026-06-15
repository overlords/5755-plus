# 支付创建按 `(gameId, cpOrderId)` 幂等;回调去重仍固定 `orderId`

状态:已接受(2026-06-16)

## 背景

`POST /api/sdk/v2/orders`(支付创建)此前对 `cpOrderId` 只校验非空与长度(`domain_m3.go`),`orders` 表主键是平台签发的 `platform_order_id`(每次调用 `"P5755"+UnixNano` 新生成),`cp_order_id` 仅 `text NOT NULL`、**无唯一约束**。契约正文亦明文确立「平台不约束 cpOrderId 唯一」的有意立场:

- 04 §4(回调段)L626/L653、05 §2.4 / §3.2:`cpOrderId`「游戏侧自保唯一,平台不强制约束」、「平台不保证全局唯一,不能作充值回调去重键」,去重键固定 `orderId`。

该立场内部自洽:正因创建侧允许同 `cpOrderId` 出多单,回调去重才**必须**用 `orderId`(否则游戏按 `cpOrderId` 去重会误丢第二单回调)。

经一次「玩家信息泄露审计」grill(威胁档位=抓包+本地取证)旁生发现:入站签名只有 ±300s 时效窗口、无 nonce/seen-cache(见 04 §1.3),**窗口内逐字节重放可通过**;叠加创建侧无 `cpOrderId` 幂等,**同一 `cpOrderId` 重放/游戏正常重试 → 平台裂出多个可独立支付的 `platformOrderId`**。虽然双发货已被下游兜底(04 §4 要求游戏服务端按 `orderId` 幂等发货 + `cpOrderId`/`amount`/`account` 交叉校验),但平台侧让一个 CP 订单裂成多个可支付订单是脏的、且放大了重放面。

## 决定

**平台在支付创建侧按 `(gameId, cpOrderId)` 幂等;充值回调去重键不变,仍固定 `orderId`。**

创建侧行为(`POST /orders`):

- DB 加 `UNIQUE(game_id, cp_order_id)`。同 `(gameId, cpOrderId)` 再次创建时:
  - 仍 `待支付` 且金额/商品/区服/角色等归属字段与原单**一致** → **返回已存在订单**(同 `orderId`/`paymentUrl`,不新建);
  - 字段**不一致**(同键改金额等)→ 拒,`reason=order_invalid`(防篡改);
  - 已 `已支付` → 拒,`reason=order_invalid`(已支付不可重复创建)。
- **作用域 `(gameId, cpOrderId)`,跨游戏不保证唯一**(不同游戏可重用同一 `cpOrderId` 串)。
- 不新增公开 `OperateCode` / `reason`,复用 `order_invalid`。

回调侧:**完全不变**——去重键仍 `orderId`(平台订单权威主键),`cpOrderId`/`amount`/`account` 仍仅作一致性交叉校验。

## 为什么

- **从源头堵重放/重复下单**:把幂等放在创建侧,平台不再裂多单;不依赖「下游游戏服务端兜底」作为唯一防线。
- **作用域选 `(gameId, cpOrderId)` 而非全局**:`gameId` 已在每行订单上;按游戏幂等既够用,又**保住「跨游戏不保证全局唯一」**这条既有口径(不同游戏的 CP 订单号空间不互相干扰),与回调段措辞不冲突。
- **回调去重键不动**:回调可能因平台自愈巡检(`RedeliverPendingCallbacks`)重投,`orderId` 是平台侧权威主键、与订单 1:1,仍是最稳的回调幂等键;游戏服务端契约零改动。
- **覆盖既有「平台不约束」立场是知情决定**:grill 中已把 L626/L653 摆出,确认其与本决定的真实冲突仅在「创建侧约束与否」一句;翻案换来的是平台侧支付完整性,代价是一处 DB 约束 + 一段查存逻辑,可接受。

## 考虑过的其他选项

- **维持「平台不约束 cpOrderId」(Y),靠游戏服务端 `orderId` 幂等兜底** — 否决。双发货确被下游挡住,但平台侧多单脏、放大重放面;把完整性全押在接入方实现上,不如平台自堵。
- **全局唯一 `cpOrderId`** — 否决。跨游戏强制唯一无必要,且与既有「跨游戏不保证唯一」口径冲突、徒增跨租户耦合。
- **创建侧也改回调去重键为 `cpOrderId`** — 否决。回调重投与 `orderId` 1:1,改键无收益且破坏游戏服务端既有契约。

## 后果

- **DB**:新增 append-only migration,`orders` 加 `UNIQUE(game_id, cp_order_id)`。
- **`domain.CreateOrder`**:INSERT 前按 `(gameId, cpOrderId)` 查存,按上述三分支处置(待支付且字段一致→返回已存在;字段不一致或已支付→`order_invalid`)。
- **文档(本 ADR 已同步)**:`04 §2.9.1`(创建幂等业务规则 + 请求字段注)、`04 §4`(L626/L653 措辞)、`05 §2.4`、`02`(cpOrderId 词条)、`integration-guide`、`server-facing-openapi.yaml` 统一为「创建侧 `(gameId,cpOrderId)` 幂等、跨游戏不保证唯一、回调去重仍按 `orderId`」。
- **测试口径**:新增「同 `(gameId,cpOrderId)` 重复创建 → 待支付返回同单 / 改金额拒 / 已支付拒」回归;原「每次创建必出新 `orderId`」的隐含假设需复核。
- **不影响**:回调链路、`orderId` 幂等去重、游戏服务端对接契约。
