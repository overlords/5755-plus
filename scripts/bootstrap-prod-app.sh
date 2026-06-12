#!/usr/bin/env bash
# CTID 106 生产应用机一次性 bootstrap(M4-S7/#40),仿 CTID 105 的 OpenRC 形态。
# 在 106 上(或经 ssh root@106 'bash -s' 管道)运行一次,把容器从空白带到「可被 deploy.sh prod 接管」。
#
# 幂等:重复运行只补齐缺失项。完成后用 scripts/deploy.sh prod 推二进制+重启。
#
# 必填环境变量:
#   PROD_DATABASE_URL   独立生产库连接串(决策:独立生产库;勿用 dev 的 m5755_v2)
# 占位先行(决策:占位密钥先验结构;拿到真值后改 /opt/m5755/.env 重启即可):
#   SIGNING_KEY_ID      默认 prod-key-placeholder
#   SIGNING_KEY_SECRET  默认 __PLACEHOLDER_SECRET__
#   CALLBACK_SECRET     默认 __PLACEHOLDER_CALLBACK__
#
# 不在本脚本职责内(以实际拓扑为准,需单独确认):
#   - 公网边缘 sdk.xingninghuyu.com 的 TLS 终止与反代 → 106:80(对应 dev 的 Debian 边缘机)。
#   - 本脚本只配 106 本机 nginx :80 → 127.0.0.1:8080,与 105 一致。
set -euo pipefail

: "${PROD_DATABASE_URL:?必须提供独立生产库 PROD_DATABASE_URL(勿用 dev 的 m5755_v2)}"
SIGNING_KEY_ID="${SIGNING_KEY_ID:-prod-key-placeholder}"
SIGNING_KEY_SECRET="${SIGNING_KEY_SECRET:-__PLACEHOLDER_SECRET__}"
CALLBACK_SECRET="${CALLBACK_SECRET:-__PLACEHOLDER_CALLBACK__}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> 1. 服务账户与目录"
id m5755 >/dev/null 2>&1 || adduser -D -H -s /sbin/nologin m5755
mkdir -p /opt/m5755
touch /var/log/m5755-server.log && chown m5755:m5755 /var/log/m5755-server.log

echo "==> 2. 环境文件 /opt/m5755/.env(占位密钥先行;真值后替换重启)"
if [ ! -f /opt/m5755/.env ]; then
  cat > /opt/m5755/.env <<EOF
PLATFORM_ENV=prod
PUBLIC_BASE_URL=https://sdk.xingninghuyu.com
DATABASE_URL=${PROD_DATABASE_URL}
SIGNING_KEY_ID=${SIGNING_KEY_ID}
SIGNING_KEY_SECRET=${SIGNING_KEY_SECRET}
CALLBACK_SECRET=${CALLBACK_SECRET}
EOF
  chmod 600 /opt/m5755/.env && chown m5755:m5755 /opt/m5755/.env
  echo "    已写入(占位密钥)。真实密钥到位后:编辑 /opt/m5755/.env,rc-service m5755-server restart"
else
  echo "    已存在,跳过(避免覆盖已注入的真实密钥)"
fi

echo "==> 3. OpenRC 服务"
install -m 0755 "$SCRIPT_DIR/openrc-m5755-server" /etc/init.d/m5755-server
rc-update add m5755-server default >/dev/null 2>&1 || true

echo "==> 4. 本机 nginx :80 → 127.0.0.1:8080"
apk info -e nginx >/dev/null 2>&1 || apk add --no-cache nginx
mkdir -p /etc/nginx/http.d
cat > /etc/nginx/http.d/m5755.conf <<'EOF'
server {
    listen 80 default_server;
    server_name sdk.xingninghuyu.com;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF
nginx -t && (rc-service nginx restart || rc-service nginx start)
rc-update add nginx default >/dev/null 2>&1 || true

echo "==> bootstrap 完成。后续:本机执行 scripts/deploy.sh prod 推 production 二进制并启动。"
echo "    若 production 因缺密钥 fail-closed 拒绝启动,属预期(占位也是非空值,应能起;真值替换后更安全)。"
