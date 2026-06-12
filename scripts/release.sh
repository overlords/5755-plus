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
