//go:build !production

package main

import (
	"context"
	"log"

	"m5755/server/internal/domain"
	"m5755/server/internal/store"
)

// bootstrapEnv dev/联调构建:保留公开测试密钥与 dev 种子(联调/回归口径)。
// 返回 (callbackSecret, realNameMock)。
func bootstrapEnv(ctx context.Context, st *store.Store, platformEnv string) (string, bool) {
	// dev 密码种子账户(密码登录回归用;production 构建不编译本函数)
	if hash, err := domain.HashPassword("Test1234"); err == nil {
		if serr := st.EnsureDevPasswordAccount(ctx, "13900000000", "密码测试账户", hash); serr != nil {
			log.Printf("dev 密码账户种子失败(忽略): %v", serr)
		}
	}
	cs := envOrDefault("CALLBACK_SECRET", "m5755-dev-callback-secret-v1")
	return cs, true // dev 实名 = mock 口径
}
