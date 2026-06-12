//go:build production

package main

import (
	"context"
	"log"
	"os"

	"m5755/server/internal/store"
)

// bootstrapEnv production 构建:fail-closed——缺生产密钥拒绝启动;
// 注入生产验签密钥并停用其它所有 key(占位/历史/dev 测试),杜绝后门;
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
	// 停用除当前 keyId 外的所有密钥(dev-test-key、prod-key-placeholder 等),杜绝残留后门。
	if err := st.DeactivateOtherSigningKeys(ctx, keyID); err != nil {
		log.Fatalf("停用历史验签密钥失败: %v", err)
	}
	return cs, false // 生产实名 = fail-closed(真实 provider 就位前)
}
