package api

import (
	"github.com/gin-gonic/gin"

	"m5755/server/internal/domain"
	"m5755/server/internal/result"
)

// 凭据请求头(04 §1.4):GET 凭据不进 query。
const (
	headerPlatformToken = "X-M5755-Platform-Token"
	headerToken         = "X-M5755-Token"
)

// registerM2Routes 注册里程碑 2 端点(#11-#14),由 NewRouter 调用。
func registerM2Routes(v2 *gin.RouterGroup, svc *domain.Service) {
	v2.GET("/account-sessions", accountCheckHandler(svc))
	v2.GET("/real-name", realNameGetHandler(svc))
	v2.POST("/real-name", realNameSubmitHandler(svc))
	v2.GET("/subaccounts", subaccountsListHandler(svc))
	v2.POST("/subaccounts", subaccountsCreateHandler(svc))
	v2.PUT("/subaccounts/default", subaccountsDefaultHandler(svc))
	v2.POST("/subaccount-sessions", subaccountLoginHandler(svc))
	v2.GET("/subaccount-sessions", subaccountCheckHandler(svc))
}

// writeValid 写会话检查类响应:接口成功 success:true;明确失效附 reason 供 SDK 分流。
func writeValid(c *gin.Context, data interface{}, valid bool, invalidReason string) {
	r := result.OK(data)
	if !valid {
		r.Reason = invalidReason
	}
	c.JSON(200, r)
}

func accountCheckHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.CheckAccountSession(c.Request.Context(),
			c.Query("gameId"), c.Query("platformAccountId"), c.GetHeader(headerPlatformToken))
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		writeValid(c, data, data.Valid, result.ReasonPlatformAccountInvalid)
	}
}

func realNameGetHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.GetRealName(c.Request.Context(),
			c.Query("gameId"), c.Query("platformAccountId"), c.GetHeader(headerPlatformToken))
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

type realNameSubmitReq struct {
	GameID            string `json:"gameId"`
	PlatformAccountID string `json:"platformAccountId"`
	PlatformToken     string `json:"platformToken"`
	RealName          string `json:"realName"`
	IDNumber          string `json:"idNumber"`
}

func realNameSubmitHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req realNameSubmitReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		data, f := svc.SubmitRealName(c.Request.Context(), req.GameID, req.PlatformAccountID, req.PlatformToken, req.RealName, req.IDNumber)
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

func subaccountsListHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.ListSubaccounts(c.Request.Context(),
			c.Query("gameId"), c.Query("platformAccountId"), c.GetHeader(headerPlatformToken))
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

type subaccountBodyReq struct {
	GameID            string `json:"gameId"`
	PlatformAccountID string `json:"platformAccountId"`
	PlatformToken     string `json:"platformToken"`
	Account           string `json:"account"`
}

func subaccountsCreateHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req subaccountBodyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		// 服务端忽略请求中的任何 isDefault 字段(04 §2.5.2)——结构体不含该字段即天然忽略。
		data, f := svc.CreateSubaccount(c.Request.Context(), req.GameID, req.PlatformAccountID, req.PlatformToken)
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

func subaccountsDefaultHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req subaccountBodyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		data, f := svc.SetDefaultSubaccount(c.Request.Context(), req.GameID, req.PlatformAccountID, req.PlatformToken, req.Account)
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

func subaccountLoginHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req subaccountBodyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			result.WriteFail(c, 400, result.ReasonParamInvalid, "请求体非法")
			return
		}
		data, f := svc.LoginSubaccount(c.Request.Context(), req.GameID, req.PlatformAccountID, req.PlatformToken, req.Account)
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		result.WriteOK(c, data)
	}
}

func subaccountCheckHandler(svc *domain.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, f := svc.CheckSubaccountSession(c.Request.Context(),
			c.Query("gameId"), c.Query("account"), c.GetHeader(headerToken))
		if f != nil {
			result.WriteFail(c, f.HTTPStatus, f.Reason, f.Message)
			return
		}
		writeValid(c, data, data.Valid, result.ReasonSubaccountInvalid)
	}
}
