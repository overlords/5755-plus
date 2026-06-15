-- grill「检查数据结构」第 3 刀:给两处缺 DB 约束的数据完整性加固。
-- 保留所有中文状态值(机器码不变)、不改 wire 契约、不迁移存量行。
-- append-only:不回改 0004(orders)/0001(subaccounts)的原始 CREATE。

-- A. 订单状态机闭合集 CHECK(与 store/order_status.go 常量严格同步)。
--    幂等:用 DO-block 守卫,pg_constraint 不存在才 ADD,避免重跑失败。
--    payment_status / callback_status 是中文显示串当机器状态,存 text;
--    无 CHECK 时 UpdateOrderStatus 能写任意串、typo 静默漏发,故落 DB 兜底。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'orders_payment_status_check'
    ) THEN
        ALTER TABLE orders ADD CONSTRAINT orders_payment_status_check
            CHECK (payment_status IN ('待支付', '已支付', '支付失败'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'orders_callback_status_check'
    ) THEN
        ALTER TABLE orders ADD CONSTRAINT orders_callback_status_check
            CHECK (callback_status IN ('未投递', '投递中', '已确认', '投递失败', '无回调地址'));
    END IF;
END
$$;

-- B. subaccounts 默认小号 partial unique 兜底:每 (platform_account_id, game_id) ≤1 个 is_default=true。
--    此前只在 SetDefaultSubaccount 事务(先清后设)维护,DB 无保证;此 partial unique 与该事务兼容、不破坏。
CREATE UNIQUE INDEX IF NOT EXISTS uq_subaccounts_default
    ON subaccounts (platform_account_id, game_id) WHERE is_default;
