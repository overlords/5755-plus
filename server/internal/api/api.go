// Package api 装配 gin 路由,把 HTTP 翻译为 domain 调用并以 ApiResult 应答。
package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/devcontrol"
	"m5755/server/internal/domain"
	"m5755/server/internal/result"
	"m5755/server/internal/signature"
	"m5755/server/internal/store"
)

// NewRouter 装配运维面、SDK 契约面(强制验签)与 dev 控制面(build tag 控制是否注册)。
// baseURL 为平台服务端公网地址,用于生成支付 paymentUrl(dev 占位支付台)。
func NewRouter(svc *domain.Service, st *store.Store, now func() time.Time, baseURL string) *gin.Engine {
	if now == nil {
		now = time.Now
	}
	r := gin.New()
	r.Use(gin.Recovery(), accessLog())

	// 运维面:无签名、只读
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "m5755 platform server ok") })
	r.GET("/openapi.json", openAPIHandler)

	// #60 入站支付链路(不走 HMAC、不在 /internal):收银台首页 GET /pay/:orderId 两构建皆注册真收银台
	// (ADR-0013:dev 注沙箱渠道即真收银台联调);dev 无渠道时降级 dev 占位页,prod 无渠道走真收银台"暂无"。
	// 收银台交互 API / return sentinel / 渠道回调接收端两端皆注册。
	hasChannels := func() bool { return svc.WechatEnabled() || svc.AlipayEnabled() }
	devcontrol.RegisterCashierPage(r, payPlaceholderHandler(svc), CashierPageHandler(svc), hasChannels)
	registerPayRoutes(r, svc)

	mw := signature.Middleware(st.LookupSigningKey, now)

	// SDK 契约面:全端点强制验签 + 主体作用域授权 + dev 故障注入中间件(生产 build 为 no-op)
	v2 := r.Group("/api/sdk/v2", mw, principalScope(), devcontrol.FaultMiddleware())
	v2.GET("/config", configHandler(svc))
	v2.POST("/sms-codes", smsCodesHandler(svc))
	v2.POST("/account-sessions", accountSessionsHandler(svc))
	registerM2Routes(v2, svc)
	registerM3Routes(v2, svc, baseURL)

	// dev 控制面:dev build 注册并复用验签;production build 为 no-op(路由不存在)
	devcontrol.Register(r, st, svc, mw)

	// 用户中心面 /api/uc/v2(ADR-0010):platformToken Bearer + CORS,**不走 HMAC**;与网关面并列。
	registerUCRoutes(r, svc)

	return r
}

// principalScope 在验签之后做端点作用域授权(#86 / ADR-0016)。
// 游戏服务端 serverKey(principal=='server')只许调登录态校验 GET /api/sdk/v2/subaccount-sessions,
// 调任何其他 v2 端点一律 403 拒绝;SDK keyId(principal=='sdk' 或空)放行所有端点。
// 在登录态校验端点上,还校验 serverKey↔game 绑定:serverKey 的归属游戏(context game_id)必须与
// 被查 gameId 一致,game_id 为空或不符一律 403(serverKey 与被查游戏不符),复用 principal_not_allowed
// (同属主体越权);SDK keyId(game_id 空、全局)不做 game 比对。
// 验签已在前置中间件通过,故拒绝用 principal_not_allowed(授权层)而非 signature_invalid(验签层)。
func principalScope() gin.HandlerFunc {
	const loginCheckPath = "/api/sdk/v2/subaccount-sessions"
	return func(c *gin.Context) {
		if c.GetString(signature.ContextKeyPrincipal) != "server" {
			c.Next()
			return
		}
		// 游戏服务端 serverKey:仅放行登录态校验 GET。
		if c.Request.Method == http.MethodGet && c.FullPath() == loginCheckPath {
			keyGameID := c.GetString(signature.ContextKeyGameID)
			if keyGameID == "" || keyGameID != c.Query("gameId") {
				result.WriteFail(c, http.StatusForbidden, result.ReasonPrincipalNotAllowed,
					"serverKey 与被查游戏不符")
				c.Abort()
				return
			}
			c.Next()
			return
		}
		result.WriteFail(c, http.StatusForbidden, result.ReasonPrincipalNotAllowed,
			"游戏服务端密钥仅可调用登录态校验端点")
		c.Abort()
	}
}

// accessLog 记录每个请求的方法/路径/状态/耗时/客户端 IP;只记路径不记 query、headers 与 body,
// 杜绝 platformToken/密钥/验证码入访问日志。跳过 /healthz 噪声(部署/LB 高频探活)。
func accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		slog.Info("http_access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latencyMs", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

func requestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

func configHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID := c.Query("gameId")
		sdkVersion := c.Query("sdkVersion")
		data, f := svc.GetConfig(c.Request.Context(), gameID, sdkVersion, requestID())
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

type smsCodesReq struct {
	GameID       string `json:"gameId"`
	LoginAccount string `json:"loginAccount"`
}

func smsCodesHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req smsCodesReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		data, f := svc.RequestSmsCode(c.Request.Context(), req.GameID, req.LoginAccount)
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

type accountSessionsReq struct {
	GameID           string `json:"gameId"`
	LoginMethod      string `json:"loginMethod"`
	LoginAccount     string `json:"loginAccount"`
	Credential       string `json:"credential"`
	ChannelID        string `json:"channelId"`
	ChannelSource    string `json:"channelSource"`
	DeviceID         string `json:"deviceId"`
	DeviceVerifyCode string `json:"deviceVerifyCode"`
}

func accountSessionsHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req accountSessionsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		data, f := svc.Login(c.Request.Context(), domain.LoginInput{
			GameID:           req.GameID,
			LoginMethod:      req.LoginMethod,
			LoginAccount:     req.LoginAccount,
			Credential:       req.Credential,
			ChannelID:        req.ChannelID,
			ChannelSource:    req.ChannelSource,
			DeviceID:         req.DeviceID,
			DeviceVerifyCode: req.DeviceVerifyCode,
		})
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

// openAPIHandler 返回最小机器契约自描述(里程碑 1 已实现端点)。
func openAPIHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"openapi": "3.0.0",
		"info":    gin.H{"title": "5755 平台服务端 SDK 网关", "version": "v2"},
		"paths": gin.H{
			"/api/sdk/v2/config":           gin.H{"get": gin.H{"summary": "初始化配置"}},
			"/api/sdk/v2/sms-codes":        gin.H{"post": gin.H{"summary": "短信验证码请求"}},
			"/api/sdk/v2/account-sessions": gin.H{"post": gin.H{"summary": "5755 账户登录"}},
		},
	})
}
