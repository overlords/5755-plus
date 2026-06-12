#!/usr/bin/env bash
# 部署 5755 平台服务端到 dev(CTID 105 / sdk-dev)或 prod(CTID 106 / sdk)。
# 模型沿用旧项目 push-dev-server.sh 思路,改为推送 Go 单二进制 + systemd。
#
# 用法:
#   scripts/deploy.sh dev      # 默认构建(注册 dev 控制面),部署 CTID 105
#   scripts/deploy.sh prod     # -tags production(/internal/* 路由不存在),部署 CTID 106
#
# 配置(放 scripts/.env.deploy,已被 .gitignore 忽略,或导出为环境变量):
#   M5755_DEPLOY_HOST=192.168.88.106     Proxmox 网关 / SSH 跳板
#   M5755_DEPLOY_USER=root
#   M5755_DEPLOY_DEV_CTID=105
#   M5755_DEPLOY_PROD_CTID=106
#   M5755_DEPLOY_APP_DIR=/opt/m5755
#   M5755_DEPLOY_MODE=pct                pct(经 Proxmox 进容器)或 direct(直接 SSH 容器)
#   M5755_DEPLOY_DIRECT_HOST=            direct 模式下容器可达地址(如 sdk-dev.xingninghuyu.com)
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
server_dir="$repo_root/server"

[[ -f "$script_dir/.env.deploy" ]] && { set -a; . "$script_dir/.env.deploy"; set +a; }

target="${1:-dev}"
host="${M5755_DEPLOY_HOST:-192.168.88.106}"
user="${M5755_DEPLOY_USER:-root}"
app_dir="${M5755_DEPLOY_APP_DIR:-/opt/m5755}"
mode="${M5755_DEPLOY_MODE:-pct}"

case "$target" in
  dev)  ctid="${M5755_DEPLOY_DEV_CTID:-105}";  build_tags="";            healthhost="sdk-dev.xingninghuyu.com" ;;
  prod) ctid="${M5755_DEPLOY_PROD_CTID:-106}"; build_tags="production";  healthhost="sdk.xingninghuyu.com" ;;
  *) echo "未知目标:$target(用 dev|prod)" >&2; exit 1 ;;
esac

echo "==> 交叉编译 linux/amd64(静态,tags='${build_tags}')"
out="$repo_root/dist/m5755-server-$target"
mkdir -p "$repo_root/dist"
( cd "$server_dir" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "$build_tags" -o "$out" ./cmd/server )
echo "    产物:$out ($(du -h "$out" | cut -f1))"

echo "==> 推送二进制到 $user@$host (CTID $ctid, mode=$mode)"
remote_tmp="/tmp/m5755-server-$target.$$"
scp -o StrictHostKeyChecking=accept-new "$out" "$user@$host:$remote_tmp"

run_remote() {
  # 在容器内执行命令:pct 模式经 Proxmox push/exec;direct 模式直接 SSH 容器。
  if [[ "$mode" == "pct" ]]; then
    ssh "$user@$host" "$@"
  else
    ssh "$user@${M5755_DEPLOY_DIRECT_HOST:?direct 模式需设 M5755_DEPLOY_DIRECT_HOST}" "$@"
  fi
}

if [[ "$mode" == "pct" ]]; then
  echo "==> 经 Proxmox 注入容器 $ctid 并重启服务"
  ssh "$user@$host" bash -se <<EOF
set -euo pipefail
pct push $ctid "$remote_tmp" "$app_dir/current/m5755-server.new"
pct exec $ctid -- bash -c '
  install -d "$app_dir/current"
  mv "$app_dir/current/m5755-server.new" "$app_dir/current/m5755-server"
  chmod +x "$app_dir/current/m5755-server"
  systemctl restart m5755-server
'
rm -f "$remote_tmp"
EOF
else
  echo "==> 直接在容器安装并重启服务"
  run_remote bash -se <<EOF
set -euo pipefail
install -d "$app_dir/current"
mv "$remote_tmp" "$app_dir/current/m5755-server"
chmod +x "$app_dir/current/m5755-server"
systemctl restart m5755-server
EOF
fi

echo "==> 健康检查 https://$healthhost/healthz"
for i in $(seq 1 15); do
  if curl -fsS "https://$healthhost/healthz" >/dev/null 2>&1; then
    echo "    OK:$(curl -fsS "https://$healthhost/healthz")"
    exit 0
  fi
  sleep 2
done
echo "    健康检查未通过,请检查容器内 systemd 日志:journalctl -u m5755-server" >&2
exit 1
