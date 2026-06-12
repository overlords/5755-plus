// Package api 装配 gin 路由,把 HTTP 翻译为 domain 调用并以 ApiResult 应答。
package api

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/devcontrol"
	"m5755/server/internal/domain"
	"m5755/server/internal/result"
	"m5755/server/internal/signature"
	"m5755/server/internal/store"
)

// NewRouter 装配运维面、SDK 契约面(强制验签)与 dev 控制面(build tag 控制是否注册)。
func NewRouter(svc *domain.Service, st *store.Store, now func() time.Time) *gin.Engine {
	if now == nil {
		now = time.Now
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// 运维面:无签名、只读
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "m5755 platform server ok") })
	r.GET("/openapi.json", openAPIHandler)

	mw := signature.Middleware(st.LookupSigningKey, now)

	// SDK 契约面:全端点强制验签
	v2 := r.Group("/api/sdk/v2", mw)
	v2.GET("/config", configHandler(svc))
	v2.POST("/sms-codes", smsCodesHandler(svc))
	v2.POST("/account-sessions", accountSessionsHandler(svc))
	registerM2Routes(v2, svc)

	// dev 控制面:dev build 注册并复用验签;production build 为 no-op(路由不存在)
	devcontrol.Register(r, st, mw)

	return r
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
	GameID        string `json:"gameId"`
	LoginMethod   string `json:"loginMethod"`
	LoginAccount  string `json:"loginAccount"`
	Credential    string `json:"credential"`
	ChannelID     string `json:"channelId"`
	ChannelSource string `json:"channelSource"`
}

func accountSessionsHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req accountSessionsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		data, f := svc.Login(c.Request.Context(), domain.LoginInput{
			GameID:        req.GameID,
			LoginMethod:   req.LoginMethod,
			LoginAccount:  req.LoginAccount,
			Credential:    req.Credential,
			ChannelID:     req.ChannelID,
			ChannelSource: req.ChannelSource,
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
