-- 里程碑 1 最小数据模型(04 契约 SDK 网关面)

CREATE TABLE IF NOT EXISTS games (
    game_id text PRIMARY KEY,
    game_name text NOT NULL,
    maintenance_enabled boolean NOT NULL DEFAULT false,
    maintenance_message text NOT NULL DEFAULT '',
    anti_addiction_entry_blocked boolean NOT NULL DEFAULT false,
    anti_addiction_payment_blocked boolean NOT NULL DEFAULT false,
    protocol_version text NOT NULL DEFAULT '1',
    config_version text NOT NULL DEFAULT '1',
    sdk_latest_version text NOT NULL DEFAULT '1.0.0',
    sdk_min_version text NOT NULL DEFAULT '1.0.0',
    login_domain text NOT NULL DEFAULT '',
    payment_domain text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- 入站验签密钥(非密 keyId + secret);dev 为公开测试密钥
CREATE TABLE IF NOT EXISTS signing_keys (
    key_id text PRIMARY KEY,
    secret text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS platform_accounts (
    platform_account_id text PRIMARY KEY,
    login_account text NOT NULL UNIQUE,
    display_name text NOT NULL DEFAULT '',
    channel_id text NOT NULL DEFAULT 'default',
    channel_source text NOT NULL DEFAULT '',
    real_name_verified boolean NOT NULL DEFAULT false,
    adult boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sms_codes (
    code_id text PRIMARY KEY,
    game_id text NOT NULL,
    login_account text NOT NULL,
    code text NOT NULL,
    provider_mode text NOT NULL DEFAULT 'mock',
    expires_at timestamptz NOT NULL,
    consumed boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sms_codes_lookup ON sms_codes (game_id, login_account, created_at DESC);

CREATE TABLE IF NOT EXISTS account_sessions (
    platform_token text PRIMARY KEY,
    platform_account_id text NOT NULL,
    game_id text NOT NULL,
    revoked boolean NOT NULL DEFAULT false,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_account_sessions_acct ON account_sessions (platform_account_id, game_id);

CREATE TABLE IF NOT EXISTS subaccounts (
    account text PRIMARY KEY,
    platform_account_id text NOT NULL,
    game_id text NOT NULL,
    display_name text NOT NULL,
    seq integer NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_subaccounts_owner ON subaccounts (platform_account_id, game_id);

-- dev 控制面注入态(覆盖层),按 gameId 作用域;生产不写入
CREATE TABLE IF NOT EXISTS dev_injections (
    game_id text PRIMARY KEY,
    maintenance_enabled boolean,
    maintenance_message text,
    updated_at timestamptz NOT NULL DEFAULT now()
);
