#!/usr/bin/env bash
# 一条命令发布流(M4-S8/#41):
#   纯净化门禁(元测试自检 + 真实 prod AAR 五维)→ 构建 dev+prod 交付 AAR →
#   构建 production 服务端二进制 → 产物落 dist/ + 可审计摘要(产物名/sha256/变体标识/git)。
# 不自动部署(部署用 scripts/deploy.sh prod);本脚本只产出并自证纯净。
# 用法:scripts/release.sh
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
android="$repo_root/android"
dist="$repo_root/dist"
mkdir -p "$dist"

# Android 构建需 JBR 21;允许调用方覆盖 JAVA_HOME
: "${JAVA_HOME:=/Applications/Android Studio.app/Contents/jbr/Contents/Home}"
export JAVA_HOME

echo "==> [1/4] 纯净化门禁(元测试自检 + 真实 prod AAR 五维,fail-closed)"
( cd "$android" && ./gradlew --no-daemon --console=plain \
    :sdk:verifyPurityGateMetaTest :sdk:verifyPublicAarPurity )

echo "==> [2/4] 构建交付 AAR(dev + prod release)"
( cd "$android" && ./gradlew --no-daemon --console=plain \
    :sdk:assembleDevRelease :sdk:assembleProdRelease )
cp "$android/sdk/build/outputs/aar/sdk-dev-release.aar"  "$dist/"
cp "$android/sdk/build/outputs/aar/sdk-prod-release.aar" "$dist/"

# prod AAR 注入真实签名密钥(从 gitignored scripts/.env.prod-secrets 取;
# 源码 src/prod/assets 永远保留 __INJECT_ 占位,真值只进 dist/ 产物,不入库)。
# 与服务端 .env 用【同一对】keyId/secret,HMAC 两端配对。
if [ -f "$repo_root/scripts/.env.prod-secrets" ]; then
  set -a; . "$repo_root/scripts/.env.prod-secrets"; set +a
  tmp=$(mktemp -d); mkdir -p "$tmp/assets"
  unzip -p "$dist/sdk-prod-release.aar" assets/m5755-sdk-platform.properties > "$tmp/assets/m5755-sdk-platform.properties"
  sed -i '' \
    -e "s|^keyId=.*|keyId=${SIGNING_KEY_ID}|" \
    -e "s|^signatureSecret=.*|signatureSecret=${SIGNING_KEY_SECRET}|" \
    -e "s|^signatureConfigVersion=.*|signatureConfigVersion=1|" \
    "$tmp/assets/m5755-sdk-platform.properties"
  ( cd "$tmp" && zip -q "$dist/sdk-prod-release.aar" assets/m5755-sdk-platform.properties )
  rm -rf "$tmp"
  if unzip -p "$dist/sdk-prod-release.aar" assets/m5755-sdk-platform.properties | grep -q "__INJECT_"; then
    echo "    ✗ prod AAR 仍含 __INJECT_ 占位,注入失败"; exit 1
  fi
  echo "    prod AAR 已注入真实 keyId=${SIGNING_KEY_ID}(与服务端配对)"
else
  echo "    ⚠ 无 scripts/.env.prod-secrets:prod AAR 保留 __INJECT_ 占位,不可用于真实签名"
fi

echo "==> [3/4] 构建 production 服务端二进制(-tags production)"
( cd "$repo_root/server" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags production -o "$dist/m5755-server-prod" ./cmd/server )

echo "==> [4/4] 校验和摘要"
manifest="$dist/release-manifest.txt"
{
  echo "# 5755 SDK v2 发布摘要"
  echo "# 生成: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "# git: $(git -C "$repo_root" rev-parse --short HEAD) ($(git -C "$repo_root" rev-parse --abbrev-ref HEAD))"
  echo "# 门禁: verifyPurityGateMetaTest + verifyPublicAarPurity 通过"
  echo
  printf "%-26s %-18s %s\n" "ARTIFACT" "VARIANT" "SHA256"
  for entry in "sdk-dev-release.aar:dev-integration" "sdk-prod-release.aar:prod-public" "m5755-server-prod:server-production"; do
    name="${entry%%:*}"; variant="${entry##*:}"
    sum=$(shasum -a 256 "$dist/$name" | awk '{print $1}')
    printf "%-26s %-18s %s\n" "$name" "$variant" "$sum"
  done
} | tee "$manifest"

echo
echo "==> 发布产物就绪:$dist"
echo "    交付公开使用方 → sdk-prod-release.aar(已过纯净化门禁)"
echo "    部署生产服务端 → scripts/deploy.sh prod(已绑 CTID 106 / sdk.xingninghuyu.com)"
