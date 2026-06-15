// Command server 是 5755 平台服务端入口:读配置、套迁移、装配路由、起 HTTP 服务。
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
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
	// 结构化业务/访问日志 → stdout(openrc 同步 stdout+stderr 至 /var/log/m5755-server.log);
	// 生命周期 log.* 仍走 stderr,同文件汇合。运维可按 orderId/account(脱敏)检索链路。
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

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

	// 构建变体决定 bootstrap:dev=种子+测试密钥+mock 短信;production=fail-closed+密钥注入+京东云短信。
	opt := bootstrapEnv(ctx, st, platformEnv)

	baseURL := envOrDefault("PUBLIC_BASE_URL", "https://sdk-dev.xingninghuyu.com")
	// #60 入站支付渠道(微信/支付宝):env 注入、fail-closed、绝不入码;
	// notify/return URL 与 paymentUrl 同源(baseURL)。未配置渠道留 nil,请求时 503。
	opt.Channels = buildPaymentChannels(baseURL)
	svc := domain.NewWith(st, opt)
	// 平台侧充值回调重投巡检:出站投递失败/投递中订单定时重投(游戏侧幂等),不依赖渠道重推 → 漏发自愈。
	go svc.RunCallbackRetryLoop(ctx, callbackRetryInterval())
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

// callbackRetryInterval 充值回调重投巡检间隔(默认 30s,env CALLBACK_RETRY_INTERVAL_SECONDS 覆盖)。
func callbackRetryInterval() time.Duration {
	if s := os.Getenv("CALLBACK_RETRY_INTERVAL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 30 * time.Second
}
