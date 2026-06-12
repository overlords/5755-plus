-- 种子数据:一个测试游戏 + 一把 dev 公开测试签名密钥(幂等)
-- 注意:dev-test-key 的 secret 是公开测试密钥,故意非机密,仅用于联调与回归。

INSERT INTO games (game_id, game_name, protocol_version, config_version, sdk_latest_version, sdk_min_version, login_domain, payment_domain)
VALUES ('m5755-demo', '样例游戏', '1', '2026.06.1', '1.0.0', '1.0.0', 'sdk-dev.xingninghuyu.com', 'sdk-dev.xingninghuyu.com')
ON CONFLICT (game_id) DO NOTHING;

INSERT INTO signing_keys (key_id, secret, active)
VALUES ('dev-test-key', 'm5755-dev-public-test-secret-v1', true)
ON CONFLICT (key_id) DO NOTHING;
