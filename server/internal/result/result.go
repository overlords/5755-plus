// Package result 定义 04 契约统一响应 ApiResult 与机器可读 reason 枚举。
package result

import "github.com/gin-gonic/gin"

// 公开业务码:SDK 归一到 OperateCode,服务端只用粗粒度 0/3/6;细分原因走 reason。
const (
	CodeSuccess = 0
	CodeFailure = 3
)

// reason 枚举(04 §1.2.1),失效分流的唯一机器可读依据。
const (
	ReasonMaintenance               = "maintenance"
	ReasonCredentialInvalid         = "credential_invalid"
	ReasonSmsCodeInvalid            = "sms_code_invalid"
	ReasonSmsCodeExpired            = "sms_code_expired"
	ReasonSmsRateLimited            = "sms_rate_limited"
	ReasonPlatformAccountInvalid    = "platform_account_invalid"
	ReasonSubaccountInvalid         = "subaccount_invalid"
	ReasonRealNameRequired          = "real_name_required"
	ReasonAntiAddictionEntryBlocked = "anti_addiction_entry_blocked"
	ReasonAntiAddictionPayBlocked   = "anti_addiction_payment_blocked"
	ReasonSubaccountLimitReached    = "subaccount_limit_reached"
	ReasonOrderInvalid              = "order_invalid"
	ReasonParamInvalid              = "param_invalid"
	ReasonSignatureInvalid          = "signature_invalid"
	ReasonTimestampExpired          = "timestamp_expired"
	ReasonPlatformUnavailable       = "platform_unavailable"
	// 里程碑 3:设备首次密码登录需短信验证(04 §1.2.1 修订随 #25 提交)。
	ReasonDeviceVerificationRequired = "device_verification_required"
	// #86 / ADR-0016:验签已通过但调用主体无权访问该端点(游戏服务端 serverKey 越权调登录态校验外的端点)。
	// 与 signature_invalid 区分——验签本身有效,是授权(端点作用域)层拒绝。
	ReasonPrincipalNotAllowed = "principal_not_allowed"
)

// ApiResult 是所有响应的统一信封。reason 在失败时必填,成功时省略(omitempty)。
type ApiResult struct {
	Success bool        `json:"success"`
	Code    int         `json:"code"`
	Reason  string      `json:"reason,omitempty"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(data interface{}) ApiResult {
	return ApiResult{Success: true, Code: CodeSuccess, Message: "ok", Data: data}
}

func Fail(reason, message string) ApiResult {
	return ApiResult{Success: false, Code: CodeFailure, Reason: reason, Message: message}
}

// WriteOK / WriteFail 写出 ApiResult。失败用 200 之外的 HTTP 状态时,响应体仍保持 ApiResult 形态(04 §1.2)。
func WriteOK(c *gin.Context, data interface{}) {
	c.JSON(200, OK(data))
}

func WriteFail(c *gin.Context, httpStatus int, reason, message string) {
	c.JSON(httpStatus, Fail(reason, message))
}
