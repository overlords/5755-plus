//go:build production

package api

import "testing"

// skipIfProd:依赖 dev 控制面(/internal/dev-control/*,生产构建 build-tag 排除)的测试在生产构建跳过。
// 这些测试用 dev 控制面注入状态(踢号/防沉迷/故障/complete-payment 等),生产构建该路由不存在,
// 故只在 dev 构建运行。生产构建专属断言见 *_prod_test.go(//go:build production)。
func skipIfProd(t *testing.T) {
	t.Skip("依赖 dev 控制面(/internal/dev-control/*),生产构建不注册,跳过")
}
