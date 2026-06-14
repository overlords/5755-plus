//go:build !production

// Package devcontrol 提供 dev 控制面端点(异常注入),仅在非 production 构建注册路由。
package devcontrol

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/domain"
	"m5755/server/internal/result"
	"m5755/server/internal/store"
)

// ---- #24 故障注入(进程内,dev-only) ----

type faultEntry struct {
	typ     string
	delayMs int
	times   int
}

var (
	faultMu       sync.Mutex
	faultRegistry = map[string]*faultEntry{} // key: gameId|endpoint
)

func faultKey(gameID, endpoint string) string { return gameID + "|" + endpoint }

// FaultMiddleware 命中(gameId, 路径)且剩余次数>0 时按 type 劫持响应并扣减。
func FaultMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		gameID := c.Query("gameId")
		if gameID == "" {
			gameID = peekBodyGameID(c)
		}
		faultMu.Lock()
		e := faultRegistry[faultKey(gameID, c.Request.URL.Path)]
		var hit *faultEntry
		if e != nil && e.times > 0 {
			cp := *e
			hit = &cp
			e.times--
			if e.times <= 0 {
				delete(faultRegistry, faultKey(gameID, c.Request.URL.Path))
			}
		}
		faultMu.Unlock()
		if hit == nil {
			c.Next()
			return
		}
		switch hit.typ {
		case "delay":
			time.Sleep(time.Duration(hit.delayMs) * time.Millisecond)
			c.Next()
		case "http500":
			c.Data(500, "text/plain", []byte("injected fault"))
			c.Abort()
		case "malformed":
			c.Data(200, "application/json", []byte("{not-json"))
			c.Abort()
		default:
			c.Next()
		}
	}
}

func setFault(gameID, endpoint, typ string, delayMs, times int) {
	faultMu.Lock()
	defer faultMu.Unlock()
	faultRegistry[faultKey(gameID, endpoint)] = &faultEntry{typ: typ, delayMs: delayMs, times: times}
}

func clearFaults(gameID string) {
	faultMu.Lock()
	defer faultMu.Unlock()
	for k := range faultRegistry {
		if strings.HasPrefix(k, gameID+"|") {
			delete(faultRegistry, k)
		}
	}
}

// peekBodyGameID 从请求体偷看 gameId(不消费 body,供 fault 中间件按游戏作用域命中)。
func peekBodyGameID(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	var probe struct {
		GameID string `json:"gameId"`
	}
	_ = json.Unmarshal(body, &probe)
	return probe.GameID
}

// RegisterCashierPage 注册收银台首页路由 GET /pay/:orderId(ADR-0013:真收银台两构建)。
// dev 构建:渠道已注入(如沙箱)→ 真收银台;无渠道 → dev 占位页(dev-control 联调兜底)。
// production 构建(见 devcontrol_prod.go):恒真收银台(无渠道时真收银台自渲染"暂无可用支付方式")。
func RegisterCashierPage(r *gin.Engine, devPlaceholder, realCashier gin.HandlerFunc, hasChannels func() bool) {
	r.GET("/pay/:orderId", func(c *gin.Context) {
		if hasChannels() {
			realCashier(c)
		} else {
			devPlaceholder(c)
		}
	})
}

// Register 在 dev/local 构建下注册 /internal/dev-control/*,复用验签中间件。
func Register(r *gin.Engine, st *store.Store, svc *domain.Service, mw gin.HandlerFunc) {
	g := r.Group("/internal/dev-control", mw)

	// #23 推进调试订单并触发回调投递
	g.POST("/complete-payment", func(c *gin.Context) {
		var req struct {
			GameID  string `json:"gameId"`
			OrderID string `json:"orderId"`
			Mode    string `json:"mode"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GameID == "" || req.OrderID == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId/orderId")
			return
		}
		mode := req.Mode
		if mode == "" {
			mode = "成功"
		}
		if f := svc.CompletePayment(c.Request.Context(), req.GameID, req.OrderID, mode); f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, gin.H{"orderId": req.OrderID, "mode": mode})
	})

	// #24 故障注入
	g.POST("/fault", func(c *gin.Context) {
		var req struct {
			GameID   string `json:"gameId"`
			Endpoint string `json:"endpoint"`
			Type     string `json:"type"`
			DelayMs  int    `json:"delayMs"`
			Times    int    `json:"times"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GameID == "" || req.Endpoint == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 gameId/endpoint")
			return
		}
		times := req.Times
		if times <= 0 {
			times = 1
		}
		setFault(req.GameID, req.Endpoint, req.Type, req.DelayMs, times)
		result.WriteOK(c, gin.H{"gameId": req.GameID, "endpoint": req.Endpoint, "type": req.Type, "times": times})
	})

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
		clearFaults(req.GameID)
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
