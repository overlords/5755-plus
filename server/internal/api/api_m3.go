package api

import (
	"html"
	"net/http"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/domain"
	"m5755/server/internal/result"
)

// registerM3Routes 注册里程碑 3 端点(#21-#22),由 NewRouter 调用。
func registerM3Routes(v2 *gin.RouterGroup, svc *domain.Service, baseURL string) {
	v2.PUT("/roles", rolesHandler(svc))
	v2.POST("/orders", orderCreateHandler(svc, baseURL))
	v2.GET("/orders/:orderId", orderQueryHandler(svc))
}

type rolesReq struct {
	GameID             string `json:"gameId"`
	Account            string `json:"account"`
	Token              string `json:"token"`
	ServerID           string `json:"serverId"`
	ServerName         string `json:"serverName"`
	RoleID             string `json:"roleId"`
	RoleName           string `json:"roleName"`
	RoleLevel          string `json:"roleLevel"`
	RoleCE             string `json:"roleCe"`
	RoleStage          string `json:"roleStage"`
	RoleRechargeAmount string `json:"roleRechargeAmount"`
	RoleGuild          string `json:"roleGuild"`
}

func rolesHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req rolesReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		f, data := svc.ReportRole(c.Request.Context(), domain.RoleInput{
			GameID: req.GameID, Account: req.Account, Token: req.Token,
			ServerID: req.ServerID, ServerName: req.ServerName, RoleID: req.RoleID,
			RoleName: req.RoleName, RoleLevel: req.RoleLevel, RoleCE: req.RoleCE,
			RoleStage: req.RoleStage, RoleRechargeAmount: req.RoleRechargeAmount, RoleGuild: req.RoleGuild,
		})
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

type orderReq struct {
	GameID     string `json:"gameId"`
	Account    string `json:"account"`
	Token      string `json:"token"`
	Amount     string `json:"amount"`
	CPOrderID  string `json:"cpOrderId"`
	Commodity  string `json:"commodity"`
	ServerID   string `json:"serverId"`
	ServerName string `json:"serverName"`
	RoleID     string `json:"roleId"`
	RoleName   string `json:"roleName"`
	RoleLevel  string `json:"roleLevel"`
}

func orderCreateHandler(svc *domain.Service, baseURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req orderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonOrderInvalid, "请求体非法")
			return
		}
		data, f := svc.CreateOrder(c.Request.Context(), domain.OrderInput{
			GameID: req.GameID, Account: req.Account, Token: req.Token, Amount: req.Amount,
			CPOrderID: req.CPOrderID, Commodity: req.Commodity, ServerID: req.ServerID, ServerName: req.ServerName,
			RoleID: req.RoleID, RoleName: req.RoleName, RoleLevel: req.RoleLevel,
		}, baseURL)
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

func orderQueryHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.QueryOrder(c.Request.Context(),
			c.Query("gameId"), c.Query("account"), c.GetHeader(headerToken), c.Param("orderId"))
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

// payPlaceholderHandler dev 占位支付台页面(无签名,浏览器 WebView 加载)。
// 仅展示订单信息与"等待支付完成";真实资金渠道属生产化里程碑,生产不部署该页。
func payPlaceholderHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := html.EscapeString(c.Param("orderId"))
		page := `<!doctype html><html lang="zh"><head><meta charset="utf-8">` +
			`<meta name="viewport" content="width=device-width,initial-scale=1">` +
			`<title>5755 支付</title></head>` +
			`<body style="font-family:sans-serif;background:#f5f5f5;margin:0;padding:32px;color:#25272b">` +
			`<h2 style="color:#25272b">5755 游戏支付</h2>` +
			`<p>订单号:` + id + `</p>` +
			`<p style="color:#777b83">这是 dev 联调占位支付台,无真实资金渠道。请用 dev 控制面 complete-payment 推进订单状态。</p>` +
			`<div style="margin-top:24px;height:48px;line-height:48px;text-align:center;background:#ffc936;color:#5d4300;border-radius:6px;font-weight:700">等待支付完成…</div>` +
			`</body></html>`
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
	}
}
