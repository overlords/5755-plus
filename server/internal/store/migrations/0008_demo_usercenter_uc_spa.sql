-- demo 指回真实用户中心页:uc SPA 已部署并验证(uc.xingninghuyu.com,有效 LE 证书;
-- 见 scripts/deploy-uc.sh + scripts/uc.nginx.conf)。
-- 0006 本就指向它;此条覆盖 orientation 分支 0007 在已部署库留下的置空(0007 是 uc 域曾不可用
-- 时退回 SDK 回退页的停摆,uc 上线后失效)。编号取 0008 以避开 0007 文件名冲突;两分支合并后
-- 按文件名序 0007(置空)→ 0008(指回)运行,终值 = uc.xingninghuyu.com。
UPDATE games SET user_center_url='https://uc.xingninghuyu.com/' WHERE game_id='m5755-demo';
