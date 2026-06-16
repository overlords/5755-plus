#!/usr/bin/env bash
# 渠道归因端到端回归(M4-S6/#39,上线阻断):
#   channel-pack 打渠道包 → 安装 → 真实登录(对线上 sdk-dev)→ 服务端账户 channel_id 命中
#   → 换无渠道母包同号复登 → 归因不被覆盖(04 口径)。
# 依赖:模拟器在线(adb)、PGHOST/PGPASSWORD(读 devCode 与断言归因)、JAVA_HOME=JBR21。
set -euo pipefail

CH="guild_e2e_test"
PKG="com.m5755.sdk.ui.sample"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APK="$ROOT/android/sample/build/outputs/apk/debug/sample-debug.apk"
PACKED="/tmp/sample-$CH.apk"
PHONE="152$(date +%s | tail -c 9)"

psqlq() { PGPASSWORD="${PGPASSWORD:?}" psql -h "${PGHOST:?}" -U postgres -d m5755_v2 -tAc "$1" | tr -d '[:space:]'; }

# 文本定位点击(uiautomator dump;contains 匹配,取首个 bounds 中心)
tap_text() {
  local txt="$1" tries="${2:-10}"
  for _ in $(seq 1 "$tries"); do
    adb shell uiautomator dump /sdcard/u.xml >/dev/null 2>&1 || true
    local b
    b=$(adb shell cat /sdcard/u.xml 2>/dev/null | tr '>' '\n' | grep -F "text=\"$txt" | grep -oE '\[[0-9]+,[0-9]+\]\[[0-9]+,[0-9]+\]' | head -1)
    if [ -n "$b" ]; then
      local x y
      x=$(echo "$b" | sed -E 's/\[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\]/\1 \3/' | awk '{print int(($1+$2)/2)}')
      y=$(echo "$b" | sed -E 's/\[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\]/\2 \4/' | awk '{print int(($1+$2)/2)}')
      adb shell input tap "$x" "$y"
      return 0
    fi
    sleep 1
  done
  echo "FAIL: 找不到控件 [$txt]"; return 1
}

# 精确文本匹配(闭合引号),规避「登录」vs「登录(login)」歧义
tap_exact() {
  local txt="$1" tries="${2:-10}"
  for _ in $(seq 1 "$tries"); do
    adb shell uiautomator dump /sdcard/u.xml >/dev/null 2>&1 || true
    local b
    b=$(adb shell cat /sdcard/u.xml 2>/dev/null | tr '>' '\n' | grep -F "text=\"$txt\"" | grep -oE '\[[0-9]+,[0-9]+\]\[[0-9]+,[0-9]+\]' | head -1)
    if [ -n "$b" ]; then
      local x y
      x=$(echo "$b" | sed -E 's/\[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\]/\1 \3/' | awk '{print int(($1+$2)/2)}')
      y=$(echo "$b" | sed -E 's/\[([0-9]+),([0-9]+)\]\[([0-9]+),([0-9]+)\]/\2 \4/' | awk '{print int(($1+$2)/2)}')
      adb shell input tap "$x" "$y"
      return 0
    fi
    sleep 1
  done
  echo "FAIL: 找不到控件(exact) [$txt]"; return 1
}

type_into() { # 点中输入框(按 hint 文本)后输入
  tap_text "$1"
  adb shell input text "$2"
}

login_flow() { # $1=phone;走 进入游戏→(协议)→登录窗→devCode→登录
  adb shell am start -n "$PKG/.MainActivity" >/dev/null
  sleep 2
  tap_text "进入游戏(init"
  sleep 4
  if adb shell cat /sdcard/u.xml 2>/dev/null | grep -q "个人信息保护引导"; then :; fi
  tap_text "登录(login" 6
  sleep 2
  adb shell uiautomator dump /sdcard/u.xml >/dev/null 2>&1
  if adb shell cat /sdcard/u.xml | grep -q "个人信息保护引导"; then
    tap_exact "同意"; sleep 2
  fi
  tap_text "我已阅读并同意"
  type_into "请输入手机号" "$1"
  tap_text "发送验证码"; sleep 3
  local code
  code=$(psqlq "select code from sms_codes where login_account='$1' and not consumed order by created_at desc limit 1")
  [ -n "$code" ] || { echo "FAIL: 未取到 devCode"; exit 1; }
  type_into "请输入验证码" "$code"
  tap_exact "登录" 5   # 模态主按钮(精确文本恰为"登录")
  sleep 4
}

echo "==> 1. 构建母包 + channel-pack 打渠道包($CH)"
( cd "$ROOT/android" && ./gradlew -q :sample:assembleDebug --console=plain >/dev/null )
( cd "$ROOT/server" && go run ./cmd/channel-pack -apk "$APK" -channel "$CH" -out "$PACKED" )

echo "==> 2. 渠道包全新安装 + 真实登录"
adb uninstall "$PKG" >/dev/null 2>&1 || true
adb install "$PACKED" >/dev/null
adb logcat -c
login_flow "$PHONE"
echo "    渠道诊断:$(adb logcat -d -s M5755Sdk | grep 'channel ' | tail -1 | sed 's/.*M5755Sdk: //')"

echo "==> 3. 断言:新账户归因 = $CH"
GOT=$(psqlq "select channel_id from platform_accounts where login_account='$PHONE'")
[ "$GOT" = "$CH" ] || { echo "FAIL: channel_id=$GOT(期望 $CH)"; exit 1; }
echo "    OK channel_id=$GOT"

echo "==> 4. 无渠道母包同号复登 → 归因不覆盖"
adb uninstall "$PKG" >/dev/null
adb install "$APK" >/dev/null
login_flow "$PHONE"
GOT2=$(psqlq "select channel_id from platform_accounts where login_account='$PHONE'")
[ "$GOT2" = "$CH" ] || { echo "FAIL: 老用户归因被覆盖为 $GOT2"; exit 1; }
echo "    OK 归因未被覆盖(仍 $CH)"

echo "PASS 渠道归因端到端全链通过"
