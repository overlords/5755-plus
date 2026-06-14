//go:build !production

package api

import "testing"

// skipIfProd:dev 构建为 no-op(dev 控制面已注册,dev-control 依赖测试照常运行)。
func skipIfProd(t *testing.T) {}
