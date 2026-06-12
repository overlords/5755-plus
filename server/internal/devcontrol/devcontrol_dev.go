//go:build !production

// Package devcontrol 提供 dev 控制面端点(异常注入),仅在非 production 构建注册路由。
package devcontrol

import (
	"github.com/gin-gonic/gin"

	"m5755/server/internal/result"
	"m5755/server/internal/store"
)

// Register 在 dev/local 构建下注册 /internal/dev-control/*,复用验签中间件。
func Register(r *gin.Engine, st *store.Store, mw gin.HandlerFunc) {
	g := r.Group("/internal/dev-control", mw)

	g.POST("/maintenance", func(c *gin.Context) {
		var req struct {
			GameID  string `json:"gameId"`
			Enabled bool   `json:"enabled"`
			Message string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GameID == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId")
			return
		}
		if err := st.SetMaintenanceInjection(c.Request.Context(), req.GameID, req.Enabled, req.Message); err != nil {
			result.WriteFail(c, 503, result.ReasonPlatformUnavailable, "注入失败")
			return
		}
		result.WriteOK(c, gin.H{"gameId": req.GameID, "maintenanceEnabled": req.Enabled})
	})

	g.POST("/reset", func(c *gin.Context) {
		var req struct {
			GameID string `json:"gameId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GameID == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId")
			return
		}
		if err := st.ClearGameInjections(c.Request.Context(), req.GameID); err != nil {
			result.WriteFail(c, 503, result.ReasonPlatformUnavailable, "重置失败")
			return
		}
		result.WriteOK(c, gin.H{"gameId": req.GameID, "reset": true})
	})

	// #11 踢号:吊销主账户会话(连带小号会话),不可逆(reset 不恢复)。
	g.POST("/kick", func(c *gin.Context) {
		var req struct {
			GameID            string `json:"gameId"`
			PlatformAccountID string `json:"platformAccountId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GameID == "" || req.PlatformAccountID == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId/platformAccountId")
			return
		}
		n, err := st.KickAccount(c.Request.Context(), req.GameID, req.PlatformAccountID)
		if err != nil {
			result.WriteFail(c, 503, result.ReasonPlatformUnavailable, "踢号失败")
			return
		}
		result.WriteOK(c, gin.H{"gameId": req.GameID, "platformAccountId": req.PlatformAccountID, "revokedSessions": n})
	})

	// #12 账户级防沉迷门禁注入:覆盖 real-name 判定。
	g.POST("/anti-addiction", func(c *gin.Context) {
		var req struct {
			GameID            string `json:"gameId"`
			PlatformAccountID string `json:"platformAccountId"`
			EntryBlocked      *bool  `json:"entryBlocked"`
			PaymentBlocked    *bool  `json:"paymentBlocked"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GameID == "" || req.PlatformAccountID == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId/platformAccountId")
			return
		}
		if err := st.SetAccountInjection(c.Request.Context(), req.GameID, req.PlatformAccountID, req.EntryBlocked, req.PaymentBlocked); err != nil {
			result.WriteFail(c, 503, result.ReasonPlatformUnavailable, "注入失败")
			return
		}
		result.WriteOK(c, gin.H{"gameId": req.GameID, "platformAccountId": req.PlatformAccountID})
	})

	// #13 小号失效:停用小号并吊销其小号令牌。
	g.POST("/invalidate-subaccount", func(c *gin.Context) {
		var req struct {
			GameID  string `json:"gameId"`
			Account string `json:"account"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GameID == "" || req.Account == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId/account")
			return
		}
		if err := st.InvalidateSubaccount(c.Request.Context(), req.GameID, req.Account); err != nil {
			result.WriteFail(c, 503, result.ReasonPlatformUnavailable, "注入失败")
			return
		}
		result.WriteOK(c, gin.H{"gameId": req.GameID, "account": req.Account, "invalidated": true})
	})

	g.GET("/state", func(c *gin.Context) {
		gameID := c.Query("gameId")
		if gameID == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId")
			return
		}
		stState, err := st.GetInjectionState(c.Request.Context(), gameID)
		if err != nil {
			result.WriteFail(c, 503, result.ReasonPlatformUnavailable, "查询失败")
			return
		}
		result.WriteOK(c, stState)
	})
}
