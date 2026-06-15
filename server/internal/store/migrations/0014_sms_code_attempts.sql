-- 短信/设备验证码 per-code 尝试上限:堵死校验端点(POST /account-sessions)无限猜测爆破。
-- append-only:不回改 0001(sms_codes 原始 CREATE);只追加 attempts 计数列。
-- attempts 由 store.ConsumeSmsCode 在猜错时自增;达 smsMaxAttempts 即把该码 consumed=true 作废,
-- 使 10^6 验证码空间不可在 5 分钟有效期窗口内无限爆破(登录码与设备码同走该函数,一处加固两条路径)。
ALTER TABLE sms_codes ADD COLUMN IF NOT EXISTS attempts int NOT NULL DEFAULT 0;
