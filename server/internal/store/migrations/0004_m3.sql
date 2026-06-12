-- 里程碑 3:角色快照、订单、充值回调配置、密码登录、设备信任

ALTER TABLE games ADD COLUMN IF NOT EXISTS callback_url text NOT NULL DEFAULT '';
ALTER TABLE platform_accounts ADD COLUMN IF NOT EXISTS password_hash text NOT NULL DEFAULT '';

-- 角色快照(05 §1):按(小号, 游戏, 区服, 角色)upsert
CREATE TABLE IF NOT EXISTS roles (
    account text NOT NULL,
    game_id text NOT NULL,
    server_id text NOT NULL,
    role_id text NOT NULL,
    server_name text NOT NULL DEFAULT '',
    role_name text NOT NULL DEFAULT '',
    role_level text NOT NULL DEFAULT '',
    role_ce text NOT NULL DEFAULT '-1',
    role_stage text NOT NULL DEFAULT '-1',
    role_recharge_amount text NOT NULL DEFAULT '-1',
    role_guild text NOT NULL DEFAULT '-1',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account, game_id, server_id, role_id)
);

-- 订单(05 §2 / 04 §2.13)
CREATE TABLE IF NOT EXISTS orders (
    platform_order_id text PRIMARY KEY,
    cp_order_id text NOT NULL,
    account text NOT NULL,
    game_id text NOT NULL,
    platform_account_id text NOT NULL,
    amount numeric(12,2) NOT NULL,
    commodity text NOT NULL,
    server_id text NOT NULL,
    server_name text NOT NULL DEFAULT '',
    role_id text NOT NULL DEFAULT '',
    role_name text NOT NULL DEFAULT '',
    role_level text NOT NULL DEFAULT '',
    payment_status text NOT NULL DEFAULT '待支付',
    callback_status text NOT NULL DEFAULT '未投递',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_orders_account ON orders (account);

-- 设备信任(05/设备验证;device_id 为安装级随机 ID,非硬件标识)
CREATE TABLE IF NOT EXISTS device_trust (
    platform_account_id text NOT NULL,
    device_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (platform_account_id, device_id)
);
