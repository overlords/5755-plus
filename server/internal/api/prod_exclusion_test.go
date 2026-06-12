//go:build production

package api

import "testing"

// 生产构建下 /internal/dev-control/* 路由不存在,探测必须 404(04 §3 三重防护①②)。
func TestDevControlAbsentInProduction(t *testing.T) {
	srv, _ := setup(t)
	resp, _ := doSigned(t, srv.URL, "GET", "/internal/dev-control/state", "gameId="+seedGame, nil, 0, false, false)
	if resp.StatusCode != 404 {
		t.Fatalf("生产构建 /internal/* 应 404,得 %d", resp.StatusCode)
	}
}
