-- ADR-0020 / 04 §2.9.1:支付创建按 (game_id, cp_order_id) 幂等。
-- 同游戏内 CP 订单号唯一,堵重放/重复下单裂出多个可支付订单;跨游戏不保证唯一。
-- 回调去重键仍为 order_id(orders 主键),本约束只作用于创建侧。
-- append-only:不回改 0004(orders)原始 CREATE。幂等:pg_constraint 守卫,可重跑。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'orders_game_cporder_unique'
    ) THEN
        ALTER TABLE orders ADD CONSTRAINT orders_game_cporder_unique UNIQUE (game_id, cp_order_id);
    END IF;
END
$$;
