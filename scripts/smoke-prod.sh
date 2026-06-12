#!/usr/bin/env bash
# 生产结构冒烟(M4-S7/#40):只读公网探测,证明 production 形态正确。
# 红线:不创建任何业务数据(08/02 生产禁造测试数据);只验结构与排除。
# 用法:scripts/smoke-prod.sh [base_url]  默认 https://sdk.xingninghuyu.com
set -euo pipefail
B="${1:-https://sdk.xingninghuyu.com}"

code()  { curl -s -o /dev/null -w "%{http_code}" -m 8 "$1"; }
codeX() { curl -s -o /dev/null -w "%{http_code}" -m 8 -X "$1" "$2"; }
pass=0; fail=0
chk() { if [ "$2" = "$3" ]; then echo "  ✓ $1"; pass=$((pass+1)); else echo "  ✗ $1 -> $2(期望 $3)"; fail=$((fail+1)); fi; }

echo "== 生产结构冒烟 $B =="
chk "/healthz 200"                  "$(code "$B/healthz")" "200"
[ "$(curl -s -m8 "$B/healthz")" = "m5755 platform server ok" ] \
  && { echo "  ✓ healthz body = 我们的服务(非占位)"; pass=$((pass+1)); } \
  || { echo "  ✗ healthz body 非预期"; fail=$((fail+1)); }
chk "/internal/dev-control/state 404(生产排除①)" "$(code "$B/internal/dev-control/state")" "404"
chk "/internal/dev-control/fault POST 404"       "$(codeX POST "$B/internal/dev-control/fault")" "404"
chk "/pay/* 404(生产排除②)"        "$(code "$B/pay/P5755smoke")" "404"
chk "/api/sdk/v2/config 无签名 401" "$(code "$B/api/sdk/v2/config")" "401"
reason=$(curl -s -m8 "$B/api/sdk/v2/config" | python3 -c "import sys,json;print(json.load(sys.stdin).get('reason',''))" 2>/dev/null || echo "")
[ "$reason" = "signature_invalid" ] \
  && { echo "  ✓ config reason=signature_invalid(验签在线)"; pass=$((pass+1)); } \
  || { echo "  ✗ config reason=$reason"; fail=$((fail+1)); }
chk "/openapi.json 200"             "$(code "$B/openapi.json")" "200"

echo "== $pass 通过 / $fail 失败 =="
[ "$fail" -eq 0 ]
