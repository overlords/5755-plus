// Command server 是 5755 平台服务端入口:读配置、套迁移、装配路由、起 HTTP 服务。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/api"
	"m5755/server/internal/domain"
	"m5755/server/internal/store"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("缺少 DATABASE_URL 环境变量")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx := context.Background()
	st, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("套用迁移失败: %v", err)
	}
	platformEnv := envOrDefault("PLATFORM_ENV", "dev")
	log.Printf("迁移已套用,平台环境=%s", platformEnv)

	// dev/联调:预置带密码的测试账户(#25 密码登录回归);生产不种子。
	if platformEnv != "prod" {
		if hash, herr := domain.HashPassword("Test1234"); herr == nil {
			if serr := st.EnsureDevPasswordAccount(ctx, "13900000000", "密码测试账户", hash); serr != nil {
				log.Printf("dev 密码账户种子失败(忽略): %v", serr)
			}
		}
	}

	baseURL := envOrDefault("PUBLIC_BASE_URL", "https://sdk-dev.xingninghuyu.com")
	svc := domain.New(st)
	r := api.NewRouter(svc, st, time.Now, baseURL)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("平台服务端监听 %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务退出: %v", err)
	}
}

func envOrDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
