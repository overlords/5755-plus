#!/usr/bin/env bash
# 部署 uc SPA(用户中心 H5 静态单页)到 uc 应用机(CTID 107)。
#
# 拓扑(同 SDK,2026-06-13):公网边缘做 TLS 终止 + 反代 → 192.168.88.243(CTID 107,
# Alpine + nginx/OpenRC)= uc 应用机,本机只服务 :80 静态。
#
# 用法:
#   scripts/deploy-uc.sh
#
# 配置(scripts/.env.deploy,已 gitignore):
#   M5755_DEPLOY_KEY=~/.ssh/m5755_deploy
#   M5755_DEPLOY_UC_HOST=192.168.88.243   # CTID 107 内网地址
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
[[ -f "$script_dir/.env.deploy" ]] && { set -a; . "$script_dir/.env.deploy"; set +a; }

key="${M5755_DEPLOY_KEY:-$HOME/.ssh/m5755_deploy}"
host="${M5755_DEPLOY_UC_HOST:?CTID 107 地址未配置(M5755_DEPLOY_UC_HOST,放 scripts/.env.deploy)}"
webroot="/var/www/uc"
ssh_opts=(-i "$key" -o BatchMode=yes -o StrictHostKeyChecking=accept-new)

echo "==> 推送静态文件 → root@$host:$webroot"
ssh "${ssh_opts[@]}" "root@$host" "mkdir -p '$webroot'"
scp "${ssh_opts[@]}" \
  "$repo_root/uc/index.html" "$repo_root/uc/styles.css" \
  "$repo_root/uc/api.js" "$repo_root/uc/app.js" \
  "root@$host:$webroot/"

echo "==> 安装 nginx 站点配置(先备份占位)+ 校验 + reload"
scp "${ssh_opts[@]}" "$script_dir/uc.nginx.conf" "root@$host:/etc/nginx/http.d/uc.conf.new"
ssh "${ssh_opts[@]}" "root@$host" '
  set -e
  cd /etc/nginx/http.d
  [ -f default.conf ] && cp -f default.conf "default.conf.pre-uc.$(date +%s)" || true
  mv -f uc.conf.new default.conf
  nginx -t
  rc-service nginx reload
  echo "    nginx reloaded"
'

echo "==> 健康检查 https://uc.xingninghuyu.com/"
curl -sS -m 10 -o /dev/null -w "    HTTP %{http_code}  ssl_verify_result=%{ssl_verify_result}\n" https://uc.xingninghuyu.com/
if curl -sS -m 10 https://uc.xingninghuyu.com/ | grep -qiE "<title|用户中心|UserCenter|app\.js"; then
  echo "    OK:首页为 uc SPA"
else
  echo "    注意:首页未匹配 uc SPA 标记,请人工核对"
fi
