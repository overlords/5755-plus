-- ADR-0016:signing_keys 加调用主体类型,区分 SDK 客户端密钥与游戏服务端 serverKey。
-- principal='sdk'(SDK 网关面、密钥焊 AAR)/ 'server'(游戏服务端 serverKey、每游戏发放,
-- 登录态校验 GET subaccount-sessions 与充值回调签名共用此 secret)。
ALTER TABLE signing_keys ADD COLUMN IF NOT EXISTS principal text NOT NULL DEFAULT 'sdk';

-- dev 固定 serverKey(公开测试密钥,仅联调):demo 游戏的游戏服务端密钥。
-- secret 与 dev 充值回调密钥一致(m5755-dev-callback-secret-v1),登录态校验 + 充值回调共用(ADR-0016)。
INSERT INTO signing_keys (key_id, secret, active, principal)
VALUES ('dev-server-key', 'm5755-dev-callback-secret-v1', true, 'server')
ON CONFLICT (key_id) DO NOTHING;
