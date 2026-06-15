-- #25 设备验证每游戏开关:v2 版本默认关(密码登录只验密码),按游戏可开。
-- append-only:不回改 0001/0004;default false = 默认关,装到任意游戏均需显式开启才走设备信任块。
-- 设备信任作用域 = (platform_account_id, device_id),device_id 为安装级随机(per-app SharedPrefs),
-- 不跨游戏:换游戏 device_id 不同需重验(由「不指纹化物理设备」隐私决定逼出)。
ALTER TABLE games ADD COLUMN IF NOT EXISTS device_verification_enabled boolean NOT NULL DEFAULT false;
