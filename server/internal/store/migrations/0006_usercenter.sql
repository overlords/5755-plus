-- 用户中心平台 H5 URL(#5):经 GET /config 下发,平台配置;不进静态协议域。
ALTER TABLE games ADD COLUMN IF NOT EXISTS user_center_url text NOT NULL DEFAULT '';
-- dev demo 指向独立用户中心 H5(占位,平台后期提供真实页);prod 由平台按需配置。
UPDATE games SET user_center_url='https://uc.xingninghuyu.com/' WHERE game_id='m5755-demo' AND user_center_url='';
