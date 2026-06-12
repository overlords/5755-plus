-- 里程碑 2:实名脱敏存储列、账户级 dev 注入(防沉迷门禁覆盖)、小号会话表

ALTER TABLE platform_accounts ADD COLUMN IF NOT EXISTS real_name_masked text NOT NULL DEFAULT '';
ALTER TABLE platform_accounts ADD COLUMN IF NOT EXISTS id_number_masked text NOT NULL DEFAULT '';

-- dev 控制面:账户级防沉迷门禁注入(覆盖 real-name 判定);生产不写入
CREATE TABLE IF NOT EXISTS dev_account_injections (
    game_id text NOT NULL,
    platform_account_id text NOT NULL,
    entry_blocked boolean,
    payment_blocked boolean,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (game_id, platform_account_id)
);

-- 游戏小号登录令牌(04 §2.7:account/token 只来自小号登录接口)
CREATE TABLE IF NOT EXISTS subaccount_sessions (
    token text PRIMARY KEY,
    account text NOT NULL,
    platform_account_id text NOT NULL,
    game_id text NOT NULL,
    revoked boolean NOT NULL DEFAULT false,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_subaccount_sessions_acct ON subaccount_sessions (account);
