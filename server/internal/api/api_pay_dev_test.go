//go:build !production

package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestCashierDevRealWithChannelsElsePlaceholder 验证 ADR-0013 reconcile(dev 构建):
// 渠道已注入 → GET /pay/:orderId 渲染真收银台;无渠道 → 降级 dev 占位页(dev-control 联调兜底)。
// 仅 dev 构建有占位页;生产无渠道走真收银台"暂无"(见 api_pay_prod_test.go),故本断言限 !production。
func TestCashierDevRealWithChannelsElsePlaceholder(t *testing.T) {
	get := func(u string) string {
		resp, err := http.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	// dev + 渠道(沙箱测试密钥)→ 真收银台
	srvCh, _, _, _, _ := setupWithChannels(t)
	oid, _ := createPaidOrder(t, srvCh)
	if page := get(srvCh.URL + "/pay/" + oid); strings.Contains(page, "dev 联调占位支付台") || !strings.Contains(page, "确认支付") {
		t.Fatalf("dev+渠道应渲染真收银台,得: %.100s", page)
	}
	// dev + 无渠道 → dev 占位页
	srv, _ := setup(t)
	oid2, _ := createPaidOrder(t, srv)
	if page := get(srv.URL + "/pay/" + oid2); !strings.Contains(page, "dev 联调占位支付台") {
		t.Fatalf("dev+无渠道应降级 dev 占位页,得: %.100s", page)
	}
}
