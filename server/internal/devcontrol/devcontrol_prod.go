//go:build production

// production 构建:dev 控制面路由不注册(不是运行时开关,而是路由不存在,04 §3 三重防护①)。
package devcontrol

import (
	"github.com/gin-gonic/gin"

	"m5755/server/internal/domain"
	"m5755/server/internal/store"
)

// Register 在 production 构建下为 no-op:/internal/* 路由不存在,探测必然 404。
func Register(_ *gin.Engine, _ *store.Store, _ *domain.Service, _ gin.HandlerFunc) {}

// FaultMiddleware 生产构建为透传中间件(无故障注入)。
func FaultMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// RegisterCashierPage production 构建:注册真实平台收银台 GET /pay/:orderId(devPlaceholder 忽略)。
// dev 占位页在生产构建不存在(探测占位页文案 404);真实收银台读订单、展示金额/商品 + 渠道选择。
func RegisterCashierPage(r *gin.Engine, _, prodCashier gin.HandlerFunc) {
	r.GET("/pay/:orderId", prodCashier)
}
