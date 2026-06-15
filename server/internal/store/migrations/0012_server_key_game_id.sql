-- ADR-0016 / grill「检查数据结构」第 1 刀:signing_keys 加 game_id,表达"这把 serverKey 属于哪个游戏"。
-- principal='server' 的 serverKey 按游戏发放(出站充值回调签名 + 登录态校验 serverKey↔game 绑定都靠它);
-- principal='sdk' 的 SDK keyId 焊 AAR、全局、不绑游戏,game_id 保持空串。
-- append-only:不回改 0001/0010。
ALTER TABLE signing_keys ADD COLUMN IF NOT EXISTS game_id text NOT NULL DEFAULT '';

-- dev 固定 serverKey(dev-server-key,principal='server')绑 demo 游戏,供 per-game 选密钥与绑定校验联调。
UPDATE signing_keys SET game_id='m5755-demo' WHERE key_id='dev-server-key';
-- dev-test-key(principal='sdk')保持 game_id=''(全局 SDK 密钥,不绑游戏),无需 UPDATE。
