package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/domain"
	"m5755/server/internal/result"
)

// 用户中心面 /api/uc/v2(ADR-0010):平台对玩家、以主账户为核心。
//
// 与 SDK 网关面(/api/sdk/v2)的关键区别:
//   - 鉴权 = platformToken Bearer(复用 X-M5755-Platform-Token 头),**不走 HMAC 验签**;
//   - origin 受控(浏览器 SPA 跨域调用,需 CORS 允许已配置的 userCenterUrl 域);
//   - 数据为主账户口径(身份/账号安全/订单/实名状态),不暴露给游戏。

// registerUCRoutes 注册用户中心面,由 NewRouter 调用。无 HMAC 中间件。
func registerUCRoutes(r *gin.Engine, svc *domain.Service) {
	uc := r.Group("/api/uc/v2", ucCORS(ucAllowedOrigins()))
	uc.GET("/profile", ucProfileHandler(svc))
	uc.GET("/orders", ucOrdersHandler(svc))
	uc.POST("/phone/sms-codes", ucPhoneSmsHandler(svc))
	uc.PUT("/phone", ucRebindPhoneHandler(svc))
	uc.POST("/password/sms-codes", ucPasswordSmsHandler(svc))
	uc.PUT("/password", ucChangePasswordHandler(svc))
	// CORS 预检:注册 OPTIONS 路由,组中间件才会对预检生效(ucCORS 内 AbortWithStatus 204)。
	uc.OPTIONS("/*any", func(c *gin.Context) { c.Status(http.StatusNoContent) })
}

// ucAllowedOrigins 读 UC_ALLOWED_ORIGINS(逗号分隔);缺省 dev 占位域。
// 非 release 模式额外放行 localhost(任意端口)便于本地预览。
func ucAllowedOrigins() []string {
	raw := os.Getenv("UC_ALLOWED_ORIGINS")
	if raw == "" {
		raw = "https://uc.xingninghuyu.com"
	}
	var out []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

func ucCORS(allowed []string) gin.HandlerFunc {
	devMode := gin.Mode() != gin.ReleaseMode
	allow := func(origin string) bool {
		if origin == "" {
			return false
		}
		for _, a := range allowed {
			if a == origin {
				return true
			}
		}
		if devMode && (strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:")) {
			return true
		}
		return false
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allow(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Content-Type, "+headerPlatformToken)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func ucProfileHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.GetUserCenterProfile(c.Request.Context(), c.GetHeader(headerPlatformToken))
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

func ucOrdersHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.GetUserCenterOrders(c.Request.Context(), c.GetHeader(headerPlatformToken), c.Query("cursor"))
		writeUC(c, data, f)
	}
}

func writeUC(c *gin.Context, data interface{}, f *domain.Fault) {
	if f != nil {
		result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
		return
	}
	result.WriteOK(c, data)
}

// 换绑手机:向新手机号发验证码。
func ucPhoneSmsHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			NewPhone string `json:"newPhone"`
		}
		_ = c.ShouldBindJSON(&body)
		data, f := svc.UCSendPhoneSms(c.Request.Context(), c.GetHeader(headerPlatformToken), body.NewPhone)
		writeUC(c, data, f)
	}
}

// 换绑手机:提交(成功不登出)。
func ucRebindPhoneHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			NewPhone string `json:"newPhone"`
			SmsCode  string `json:"smsCode"`
		}
		_ = c.ShouldBindJSON(&body)
		f := svc.UCRebindPhone(c.Request.Context(), c.GetHeader(headerPlatformToken), body.NewPhone, body.SmsCode)
		writeUC(c, gin.H{"ok": true}, f)
	}
}

// 改密:向已绑手机发验证码(无 body)。
func ucPasswordSmsHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.UCSendPasswordSms(c.Request.Context(), c.GetHeader(headerPlatformToken))
		writeUC(c, data, f)
	}
}

// 改密:提交(成功作废全部会话 → SPA session_invalid 重登)。
func ucChangePasswordHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			SmsCode     string `json:"smsCode"`
			NewPassword string `json:"newPassword"`
		}
		_ = c.ShouldBindJSON(&body)
		f := svc.UCChangePassword(c.Request.Context(), c.GetHeader(headerPlatformToken), body.SmsCode, body.NewPassword)
		writeUC(c, gin.H{"ok": true}, f)
	}
}
