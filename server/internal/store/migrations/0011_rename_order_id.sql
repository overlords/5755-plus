-- ADR-0016 充值回调体定形:platformOrderId → orderId 全仓改名,DB 列同步。
-- 02 术语已立词(裸词 orderId 本归平台,platform* 前缀只留给被小号占用的 account/token)。
-- append-only:0004/0009 CREATE 的 platform_order_id 由本迁移重命名,不回改原始 CREATE。

-- orders 表 PK 列。RENAME COLUMN 自动带主键约束,无需重建 PK。
ALTER TABLE orders RENAME COLUMN platform_order_id TO order_id;

-- payment_notifications 表复合 PK (channel, platform_order_id) 的一列。
-- RENAME COLUMN 自动带复合主键约束,PK 仍为 (channel, order_id)。
ALTER TABLE payment_notifications RENAME COLUMN platform_order_id TO order_id;
