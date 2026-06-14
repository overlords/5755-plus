-- #60 入站支付链路:收银台所选渠道 + 渠道回调幂等去重

-- 订单记录玩家在收银台所选支付方式(方式无关订单的"用什么付"落库,仅用于对账/诊断;
-- 04 契约 / Order 入参保持方式无关,见 #58a ADR-0012)。
ALTER TABLE orders ADD COLUMN IF NOT EXISTS payment_method text NOT NULL DEFAULT '';

-- 渠道回调幂等:首条 notify 插入成功即"认领"该订单的发放;
-- 重复 notify 因 (channel, platform_order_id) 唯一约束插入失败 → 已处理 → 直接 ACK。
CREATE TABLE IF NOT EXISTS payment_notifications (
    channel text NOT NULL,                 -- wechat / alipay
    platform_order_id text NOT NULL,       -- = 渠道 out_trade_no
    channel_txn_id text NOT NULL DEFAULT '', -- 渠道流水号(transaction_id / trade_no),仅诊断
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel, platform_order_id)
);
