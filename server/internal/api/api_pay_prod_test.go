//go:build production

package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestCashierPageRendersInProduction 验证生产构建下 GET /pay/:orderId 渲染真实收银台
// (展示金额/商品 + 微信|支付宝选择 + 确认支付),而非 dev 占位页。
// dev 构建该路由是占位页,不在此断言(见 api_pay_test.go 注释)。
func TestCashierPageRendersInProduction(t *testing.T) {
	srv, _, _, _, _ := setupWithChannels(t)
	orderID, amount := createPaidOrder(t, srv)
	resp, err := http.Get(srv.URL + "/pay/" + orderID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	page := string(b)
	if resp.StatusCode != 200 {
		t.Fatalf("收银台页应 200,得 %d", resp.StatusCode)
	}
	for _, want := range []string{orderID, amount, "微信支付", "支付宝", "确认支付"} {
		if !strings.Contains(page, want) {
			t.Fatalf("收银台页缺 %q", want)
		}
	}
	if strings.Contains(page, "dev 联调占位支付台") {
		t.Fatalf("生产收银台不得渲染 dev 占位文案")
	}
}
