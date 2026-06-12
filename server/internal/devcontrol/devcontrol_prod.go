//go:build production

// production 构建:dev 控制面路由不注册(不是运行时开关,而是路由不存在,04 §3 三重防护①)。
package devcontrol

import (
	"github.com/gin-gonic/gin"

	"m5755/server/internal/store"
)

// Register 在 production 构建下为 no-op:/internal/* 路由不存在,探测必然 404。
func Register(_ *gin.Engine, _ *store.Store, _ gin.HandlerFunc) {}
