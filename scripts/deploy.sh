#!/usr/bin/env bash
# 部署 5755 平台服务端。
#
# 实际拓扑(2026-06-12 实测):
#   公网边缘:sdk-dev.xingninghuyu.com = nginx 边缘机(Debian,持有 LE 证书),
#            反代 → 192.168.88.241(CTID 105,Alpine 3.23 + OpenRC)= dev 应用机。
#   应用机上:本机 nginx :80 → 127.0.0.1:8080 的 m5755-server(OpenRC 服务,
#            环境文件 /opt/m5755/.env,日志 /var/log/m5755-server.log)。
#
# 用法:
#   scripts/deploy.sh dev    # 默认构建(含 dev 控制面),推送 192.168.88.241 并重启
#   scripts/deploy.sh prod   # -tags production 构建;生产应用机地址需先配置
#
# 配置(可放 scripts/.env.deploy,已 gitignore):
#   M5755_DEPLOY_KEY=~/.ssh/m5755_deploy
#   M5755_DEPLOY_DEV_HOST=192.168.88.241
#   M5755_DEPLOY_PROD_HOST=          # 生产应用机(CTID 106 内网地址),用前必填
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
[[ -f "$script_dir/.env.deploy" ]] && { set -a; . "$script_dir/.env.deploy"; set +a; }

target="${1:-dev}"
key="${M5755_DEPLOY_KEY:-$HOME/.ssh/m5755_deploy}"
case "$target" in
  dev)  host="${M5755_DEPLOY_DEV_HOST:-192.168.88.241}"; build_tags="";           health="https://sdk-dev.xingninghuyu.com/healthz" ;;
  prod) host="${M5755_DEPLOY_PROD_HOST:?生产应用机地址未配置(M5755_DEPLOY_PROD_HOST)}"; build_tags="production"; health="https://sdk.xingninghuyu.com/healthz" ;;
  *) echo "用法:scripts/deploy.sh [dev|prod]" >&2; exit 1 ;;
esac

echo "==> 交叉编译 linux/amd64 静态二进制(tags='${build_tags}')"
out="$repo_root/dist/m5755-server-$target"
mkdir -p "$repo_root/dist"
( cd "$repo_root/server" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "$build_tags" -o "$out" ./cmd/server )
echo "    产物:$out ($(du -h "$out" | cut -f1))"

echo "==> 推送到 root@$host 并原子替换 + 重启 OpenRC 服务"
scp -i "$key" -o BatchMode=yes -o StrictHostKeyChecking=accept-new "$out" "root@$host:/opt/m5755/m5755-server.new"
ssh -i "$key" -o BatchMode=yes "root@$host" 'sh -c "
  chmod 755 /opt/m5755/m5755-server.new &&
  mv /opt/m5755/m5755-server.new /opt/m5755/m5755-server &&
  rc-service m5755-server restart &&
  sleep 1 && rc-service m5755-server status
"'

echo "==> 健康检查 $health"
for i in $(seq 1 15); do
  if body=$(curl -fsS -m 5 "$health" 2>/dev/null); then
    echo "    OK:$body"; exit 0
  fi
  sleep 2
done
echo "    健康检查未通过;登录应用机查看:tail /var/log/m5755-server.log" >&2
exit 1
