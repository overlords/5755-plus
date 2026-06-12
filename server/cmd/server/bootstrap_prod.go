//go:build production

package main

import (
	"context"
	"log"
	"os"

	"m5755/server/internal/store"
)

// bootstrapEnv production 构建:fail-closed——缺生产密钥拒绝启动;
// 停用 dev 公开测试签名密钥(迁移种子对生产库的兜底),注入生产验签密钥;
// 不种任何 dev 测试账户;实名 mock 关闭(未配置真实 provider 时实名提交明确失败)。
func bootstrapEnv(ctx context.Context, st *store.Store, platformEnv string) (string, bool) {
	cs := os.Getenv("CALLBACK_SECRET")
	keyID := os.Getenv("SIGNING_KEY_ID")
	secret := os.Getenv("SIGNING_KEY_SECRET")
	if cs == "" || keyID == "" || secret == "" {
		log.Fatal("生产启动缺少 CALLBACK_SECRET / SIGNING_KEY_ID / SIGNING_KEY_SECRET(fail-closed)")
	}
	if err := st.UpsertSigningKey(ctx, keyID, secret); err != nil {
		log.Fatalf("生产验签密钥注入失败: %v", err)
	}
	if err := st.DeactivateSigningKey(ctx, "dev-test-key"); err != nil {
		log.Fatalf("停用 dev 测试密钥失败: %v", err)
	}
	return cs, false // 生产实名 = fail-closed(真实 provider 就位前)
}
