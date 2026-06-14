// Package api 装配 gin 路由,把 HTTP 翻译为 domain 调用并以 ApiResult 应答。
package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
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
	// dev 占位支付台页面(无签名;production 构建不注册 → 404,M4-S3)
	devcontrol.RegisterPayPlaceholder(r, payPlaceholderHandler(svc))

	mw := signature.Middleware(st.LookupSigningKey, now)

	// SDK 契约面:全端点强制验签 + dev 故障注入中间件(生产 build 为 no-op)
	v2 := r.Group("/api/sdk/v2", mw, devcontrol.FaultMiddleware())
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
