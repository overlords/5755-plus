package api

import (
	"html"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"m5755/server/internal/domain"
	"m5755/server/internal/result"
)

// registerPayRoutes 注册 #60 入站支付链路中**两端皆注册**的路由(收银台交互 API + return sentinel
// + 渠道回调接收端)。这些路由**不走 HMAC、不在 /internal 下、生产构建注册**(面向公网/渠道服务器)。
// 收银台首页 GET /pay/:orderId 不在此注册——它与 dev 占位页共享 path,靠 build-tag 二选一
// (devcontrol.RegisterCashierPage:dev=占位页 / prod=真实收银台),见 NewRouter。
func registerPayRoutes(r *gin.Engine, svc *domain.Service) {
	r.POST("/pay/begin", cashierBeginHandler(svc))
	r.GET("/pay/return", payReturnHandler())
	r.POST("/pay/wxnotify", wechatNotifyHandler(svc))
	r.POST("/pay/alinotify", alipayNotifyHandler(svc))
}

// CashierPageHandler 暴露真实收银台首页 handler,供 devcontrol build-tag 在生产构建注册到 /pay/:orderId。
func CashierPageHandler(svc *domain.Service) gin.HandlerFunc {
	return cashierPageHandler(svc)
}

// cashierPageHandler 渲染平台收银台 H5(类比 uc SPA):展示金额/商品 + 微信|支付宝选择。
// 收银台是平台服务端渲染的 H5,允许出现"微信/支付宝"字样(07 §0.2 AAR 禁词不适用于平台收银台)。
func cashierPageHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID := c.Param("orderId")
		co, f := svc.GetCashierOrder(c.Request.Context(), orderID)
		if f != nil {
			c.Data(f.HTTPStatus, "text/html; charset=utf-8", []byte(cashierErrorPage(f.Message)))
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderCashierPage(co)))
	}
}

// cashierBeginHandler 收银台内"确认支付"调用:按所选方式预下单(A2),返回拉起方式。
// 表单/JSON 均可:orderId + method(wechat|alipay)。
func cashierBeginHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID := c.PostForm("orderId")
		method := c.PostForm("method")
		if orderID == "" || method == "" {
			// 兼容 JSON
			var body struct {
				OrderID string `json:"orderId"`
				Method  string `json:"method"`
			}
			_ = c.ShouldBindJSON(&body)
			if body.OrderID != "" {
				orderID = body.OrderID
			}
			if body.Method != "" {
				method = body.Method
			}
		}
		if orderID == "" || method == "" {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "缺少 orderId/method")
			return
		}
		res, f := svc.BeginPayment(c.Request.Context(), orderID, method, c.ClientIP())
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		switch res.Kind {
		case "url":
			result.WriteOK(c, gin.H{"kind": "url", "redirectUrl": res.RedirectURL})
		case "jsapi":
			result.WriteOK(c, gin.H{"kind": "jsapi", "payParams": res.JSAPIPayParams})
		default:
			result.WriteFail(c, 500, result.ReasonPlatformUnavailable, "未知拉起方式")
		}
	}
}

// payReturnHandler 服务收银台 sentinel return URL 的兜底页(SDK 按 path 前缀 /pay/return 拦截,
// 通常不会真实加载到本页;仅在 SDK 未拦截时给玩家一个无害终态页)。
// status=handed|canceled 仅驱动 SDK 的 UI 口径,绝不据此发货(发货走 webhook→CompletePayment)。
func payReturnHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.Query("status")
		msg := "支付已结束,请返回游戏。"
		if status == "handed" {
			msg = "支付已提交,到账以游戏内充值回调为准。"
		} else if status == "canceled" {
			msg = "支付未完成。"
		}
		page := `<!doctype html><html lang="zh"><head><meta charset="utf-8">` +
			`<meta name="viewport" content="width=device-width,initial-scale=1">` +
			`<title>5755 支付</title></head>` +
			`<body style="font-family:sans-serif;background:#f5f5f5;margin:0;padding:32px;color:#25272b;text-align:center">` +
			`<p style="margin-top:48px">` + html.EscapeString(msg) + `</p></body></html>`
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
	}
}

// wechatNotifyHandler 微信 APIv3 回调接收端(POST /pay/wxnotify)。
// 验签/解密/反欺诈/幂等/CompletePayment 全在 domain;按结果回微信要求的 ACK。
func wechatNotifyHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "读取请求体失败"})
			return
		}
		out := svc.HandleWechatNotify(
			c.Request.Context(), raw,
			c.GetHeader("Wechatpay-Timestamp"),
			c.GetHeader("Wechatpay-Nonce"),
			c.GetHeader("Wechatpay-Signature"),
		)
		if out.OK {
			// 微信要求 HTTP 200 + {code:SUCCESS} 止重推。
			c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
			return
		}
		// 非 200 + {code:FAIL} → 微信按失败重推。
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": out.Message})
	}
}

// alipayNotifyHandler 支付宝异步通知接收端(POST /pay/alinotify)。
// 支付宝以 application/x-www-form-urlencoded POST;成功须回纯文本 "success" 止重推。
func alipayNotifyHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.Request.ParseForm(); err != nil {
			c.String(http.StatusBadRequest, "failure")
			return
		}
		params := make(map[string]string, len(c.Request.PostForm))
		for k, v := range c.Request.PostForm {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
		out := svc.HandleAlipayNotify(c.Request.Context(), params)
		if out.OK {
			c.String(http.StatusOK, "success")
			return
		}
		// 支付宝:回非 "success"(如 "failure")则继续重推。
		c.String(http.StatusOK, "failure")
	}
}
