-- NPPA 防沉迷实名核验(ADR-0007):凭据 per-game 由接入者授权;账户加异步"认证中"态。
ALTER TABLE games ADD COLUMN IF NOT EXISTS nppa_app_id text NOT NULL DEFAULT '';
ALTER TABLE games ADD COLUMN IF NOT EXISTS nppa_biz_id text NOT NULL DEFAULT '';
ALTER TABLE games ADD COLUMN IF NOT EXISTS nppa_secret_key text NOT NULL DEFAULT '';

ALTER TABLE platform_accounts ADD COLUMN IF NOT EXISTS real_name_pending boolean NOT NULL DEFAULT false;
ALTER TABLE platform_accounts ADD COLUMN IF NOT EXISTS real_name_ai text NOT NULL DEFAULT '';
ALTER TABLE platform_accounts ADD COLUMN IF NOT EXISTS real_name_pi text NOT NULL DEFAULT '';
