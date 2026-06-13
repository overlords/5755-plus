-- demo 游戏改用 SDK 内置回退用户中心页。
-- 0006 给 demo 设了占位 URL https://uc.xingninghuyu.com/,但该域证书不匹配、无可服务页面,
-- WebView 加载失败 → 用户中心空白。真实平台用户中心 H5 在本仓外、尚未就绪;在此之前 dev/demo
-- 置空 userCenterUrl,SDK 回退到内置最小页(切换小号 / 退出登录,可用、可验收)。
-- prod 游戏仍由平台按需配置真实 URL,不受影响。
UPDATE games SET user_center_url='' WHERE game_id='m5755-demo';
