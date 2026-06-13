package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"m5755/server/internal/store"
)

// ucGet 发裸 GET(用户中心面不走 HMAC 验签);可选 platformToken 头。
func ucGet(t *testing.T, base, path, token string) (*http.Response, apiResp) {
	t.Helper()
	req, err := http.NewRequest("GET", base+path, nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if token != "" {
		req.Header.Set("X-M5755-Platform-Token", token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var ar apiResp
	_ = json.Unmarshal(body, &ar)
	return res, ar
}

// 用户中心面无 HMAC:裸 token 即可取主账户身份(ADR-0010 / 06a §3)。
func TestUserCenterProfile_OK(t *testing.T) {
	srv, _ := setup(t)
	_, _, token, firstAccount := loginNewUser(t, srv)

	res, ar := ucGet(t, srv.URL, "/api/uc/v2/profile", token)
	if res.StatusCode != 200 || !ar.Success {
		t.Fatalf("期望 200/success,得到 %d / %+v", res.StatusCode, ar)
	}
	if ar.Data["nickname"] == nil || ar.Data["nickname"] == "" {
		t.Errorf("nickname 应非空: %+v", ar.Data)
	}
	if mp, _ := ar.Data["maskedPhone"].(string); len(mp) == 0 {
		t.Errorf("maskedPhone 应非空: %+v", ar.Data)
	}
	if st, _ := ar.Data["realNameStatus"].(string); st != "verified" && st != "unverified" {
		t.Errorf("realNameStatus 取值非法: %q", st)
	}
	// 新用户登录后平台保障首个小号,currentSubAccount 应回显该小号。
	sub, ok := ar.Data["currentSubAccount"].(map[string]interface{})
	if !ok {
		t.Fatalf("currentSubAccount 缺失: %+v", ar.Data)
	}
	if sub["account"] != firstAccount {
		t.Errorf("currentSubAccount.account 期望 %q,得到 %v", firstAccount, sub["account"])
	}
}

// 无 token / 失效 token → 401 + platform_account_invalid(SPA 据此 session_invalid)。
func TestUserCenterProfile_Invalid(t *testing.T) {
	srv, _ := setup(t)

	for _, tok := range []string{"", "bogus-token-xxx"} {
		res, ar := ucGet(t, srv.URL, "/api/uc/v2/profile", tok)
		if res.StatusCode != 401 {
			t.Errorf("token=%q 期望 401,得到 %d", tok, res.StatusCode)
		}
		if ar.Reason != "platform_account_invalid" {
			t.Errorf("token=%q 期望 reason=platform_account_invalid,得到 %q", tok, ar.Reason)
		}
	}
}

// CORS 预检:允许域回 ACAO,OPTIONS 返回 204。
func TestUserCenterProfile_CORSPreflight(t *testing.T) {
	srv, _ := setup(t)

	req, _ := http.NewRequest("OPTIONS", srv.URL+"/api/uc/v2/profile", nil)
	req.Header.Set("Origin", "http://localhost:8080") // dev 模式放行 localhost
	req.Header.Set("Access-Control-Request-Method", "GET")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("预检失败: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS 期望 204,得到 %d", res.StatusCode)
	}
	if res.Header.Get("Access-Control-Allow-Origin") != "http://localhost:8080" {
		t.Errorf("ACAO 应回显 localhost origin,得到 %q", res.Header.Get("Access-Control-Allow-Origin"))
	}
}

// GET /orders:凭 token 返回主账户充值订单,真实货币 CNY、字段映射正确(06a §3)。
func TestUserCenterOrders_OK(t *testing.T) {
	srv, st := setup(t)
	_, paID, token, first := loginNewUser(t, srv)

	if err := st.CreateOrder(context.Background(), store.Order{
		PlatformOrderID: "UO_uc_test_1", CPOrderID: "cp_uc_1", Account: first, GameID: seedGame,
		PlatformAccountID: paID, Amount: "648.00", Commodity: "6480 元宝", ServerID: "s1",
	}); err != nil {
		t.Fatalf("seed 订单失败: %v", err)
	}

	res, ar := ucGet(t, srv.URL, "/api/uc/v2/orders", token)
	if res.StatusCode != 200 || !ar.Success {
		t.Fatalf("期望 200/success,得到 %d / %+v", res.StatusCode, ar)
	}
	orders, _ := ar.Data["orders"].([]interface{})
	if len(orders) == 0 {
		t.Fatalf("应返回 seed 的订单: %+v", ar.Data)
	}
	o0, _ := orders[0].(map[string]interface{})
	if o0["orderId"] != "UO_uc_test_1" {
		t.Errorf("orderId 不符: %+v", o0)
	}
	if o0["productName"] != "6480 元宝" {
		t.Errorf("productName 应=commodity: %+v", o0)
	}
	if o0["currency"] != "CNY" {
		t.Errorf("currency 应=CNY: %+v", o0)
	}
}

// 无 token → 401 platform_account_invalid(失效收口,06a §3)。
func TestUserCenterOrders_NoToken_401(t *testing.T) {
	srv, _ := setup(t)
	res, ar := ucGet(t, srv.URL, "/api/uc/v2/orders", "")
	if res.StatusCode != 401 || ar.Reason != "platform_account_invalid" {
		t.Fatalf("无 token 应 401/platform_account_invalid,得到 %d / %+v", res.StatusCode, ar)
	}
}
