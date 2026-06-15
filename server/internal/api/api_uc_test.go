package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"m5755/server/internal/store"
)

// ucReq 发带 body 的请求(用户中心面不走 HMAC);可选 platformToken 头。
func ucReq(t *testing.T, base, method, path, token string, body interface{}) (*http.Response, apiResp) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, base+path, rdr)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	if token != "" {
		req.Header.Set("X-M5755-Platform-Token", token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	bb, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var ar apiResp
	_ = json.Unmarshal(bb, &ar)
	return res, ar
}

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

// CORS 预检:允许域回 ACAO,OPTIONS 返回 204,且放行自定义 token 头。
// 覆盖两类允许 origin:dev 放行的 localhost,以及生产 uc SPA 域(ADR-0010 选②绝对域
// 跨域调用的命脉——SPA 在 uc.* 调平台服务端 /api/uc/v2,全靠这条回显成立)。
func TestUserCenterProfile_CORSPreflight(t *testing.T) {
	srv, _ := setup(t)

	for _, origin := range []string{"http://localhost:8080", "https://uc.xingninghuyu.com"} {
		req, _ := http.NewRequest("OPTIONS", srv.URL+"/api/uc/v2/profile", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", "x-m5755-platform-token, content-type")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("预检失败(%s): %v", origin, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Errorf("origin=%s OPTIONS 期望 204,得到 %d", origin, res.StatusCode)
		}
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin=%s ACAO 应回显该 origin,得到 %q", origin, got)
		}
		if ah := res.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(ah, headerPlatformToken) {
			t.Errorf("origin=%s 预检应放行 %s 头,得到 %q", origin, headerPlatformToken, ah)
		}
	}
}

// GET /orders:凭 token 返回主账户充值订单,真实货币 CNY、字段映射正确(06a §3)。
func TestUserCenterOrders_OK(t *testing.T) {
	srv, st := setup(t)
	_, paID, token, first := loginNewUser(t, srv)

	oid := "UO_uc_" + paID // 唯一主键(随新用户 paID),避免共享 DB 重跑撞 23505
	if err := st.CreateOrder(context.Background(), store.Order{
		PlatformOrderID: oid, CPOrderID: "cp_" + paID, Account: first, GameID: seedGame,
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
	if o0["orderId"] != oid {
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

// 换绑手机:发码(新号)→ devCode → 提交 → profile 反映新号尾4位;成功不登出(06a §49)。
func TestUserCenterRebindPhone_OK(t *testing.T) {
	srv, _ := setup(t)
	_, _, token, _ := loginNewUser(t, srv)
	newPhone := randomPhone()

	_, ar := ucReq(t, srv.URL, "POST", "/api/uc/v2/phone/sms-codes", token, map[string]string{"newPhone": newPhone})
	if !ar.Success {
		t.Fatalf("向新号发码应成功: %+v", ar)
	}
	devCode, _ := ar.Data["devCode"].(string)
	if devCode == "" {
		t.Fatalf("mock 模式应返回 devCode: %+v", ar.Data)
	}

	res, ar2 := ucReq(t, srv.URL, "PUT", "/api/uc/v2/phone", token, map[string]string{"newPhone": newPhone, "smsCode": devCode})
	if res.StatusCode != 200 || !ar2.Success {
		t.Fatalf("换绑应 200/success,得到 %d / %+v", res.StatusCode, ar2)
	}

	// 同一 token 仍有效(换绑不登出),profile 反映新号尾4位。
	pres, pr := ucGet(t, srv.URL, "/api/uc/v2/profile", token)
	if pres.StatusCode != 200 {
		t.Fatalf("换绑后旧 token 应仍有效(不登出),得到 %d", pres.StatusCode)
	}
	mp, _ := pr.Data["maskedPhone"].(string)
	if !strings.HasSuffix(mp, newPhone[len(newPhone)-4:]) {
		t.Errorf("maskedPhone 应反映新号尾4位 %q,得到 %q", newPhone[len(newPhone)-4:], mp)
	}
}

// 换绑到已被占用的号 → 409 param_invalid(login_account 唯一约束 23505 收口)。
func TestUserCenterRebindPhone_Occupied_409(t *testing.T) {
	srv, _ := setup(t)
	_, _, tokenA, _ := loginNewUser(t, srv)
	phoneB, _, _, _ := loginNewUser(t, srv) // B 已占用 phoneB

	_, ar := ucReq(t, srv.URL, "POST", "/api/uc/v2/phone/sms-codes", tokenA, map[string]string{"newPhone": phoneB})
	if !ar.Success {
		t.Fatalf("发码应成功: %+v", ar)
	}
	devCode, _ := ar.Data["devCode"].(string)

	res, ar2 := ucReq(t, srv.URL, "PUT", "/api/uc/v2/phone", tokenA, map[string]string{"newPhone": phoneB, "smsCode": devCode})
	if res.StatusCode != 409 || ar2.Reason != "param_invalid" {
		t.Fatalf("占用号应 409/param_invalid,得到 %d / %+v", res.StatusCode, ar2)
	}
}

// 改密:发码(绑定手机)→ devCode → 提交 → 旧 token 全部失效(06a §48 处处重登)。
func TestUserCenterChangePassword_RevokesSessions(t *testing.T) {
	srv, _ := setup(t)
	_, _, token, _ := loginNewUser(t, srv)

	_, ar := ucReq(t, srv.URL, "POST", "/api/uc/v2/password/sms-codes", token, nil)
	if !ar.Success {
		t.Fatalf("发码应成功: %+v", ar)
	}
	devCode, _ := ar.Data["devCode"].(string)
	if devCode == "" {
		t.Fatalf("mock 模式应返回 devCode: %+v", ar.Data)
	}

	res, ar2 := ucReq(t, srv.URL, "PUT", "/api/uc/v2/password", token, map[string]string{"smsCode": devCode, "newPassword": "NewPass123"})
	if res.StatusCode != 200 || !ar2.Success {
		t.Fatalf("改密应 200/success,得到 %d / %+v", res.StatusCode, ar2)
	}

	// 改密后旧 token 应被作废 → profile 401 platform_account_invalid(SPA 据此 session_invalid)。
	pres, par := ucGet(t, srv.URL, "/api/uc/v2/profile", token)
	if pres.StatusCode != 401 || par.Reason != "platform_account_invalid" {
		t.Fatalf("改密后旧 token 应失效 401/platform_account_invalid,得到 %d / %+v", pres.StatusCode, par)
	}
}

// 改密后新密码必须能在网关面密码登录、错误密码被拒——锁住「uc 改密 → 密码登录」整条链路。
// (本回归由一次诊断催生:曾疑「改密成功但新密码登录失败」;实证链路正常,补此用例防未来回退。)
func TestUserCenterChangePassword_NewPasswordLogsIn(t *testing.T) {
	srv, _ := setup(t)
	phone, _, token, _ := loginNewUser(t, srv) // 验证码登录建账户,初始无密码

	_, ar := ucReq(t, srv.URL, "POST", "/api/uc/v2/password/sms-codes", token, nil)
	if !ar.Success {
		t.Fatalf("发码应成功: %+v", ar)
	}
	devCode, _ := ar.Data["devCode"].(string)
	if devCode == "" {
		t.Fatalf("mock 模式应返回 devCode: %+v", ar.Data)
	}

	const newPassword = "NewPass456"
	res, ar2 := ucReq(t, srv.URL, "PUT", "/api/uc/v2/password", token, map[string]string{"smsCode": devCode, "newPassword": newPassword})
	if res.StatusCode != 200 || !ar2.Success {
		t.Fatalf("改密应 200/success,得到 %d / %+v", res.StatusCode, ar2)
	}

	// 关键断言:新密码能在网关面密码登录(证明改密真写入、且登录读取的是同一行)。
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "loginMethod": "password", "loginAccount": phone, "credential": newPassword,
	})
	lres, lar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", lb, 0, false, false)
	if lres.StatusCode != 200 || !lar.Success {
		t.Fatalf("新密码应能密码登录,得到 %d / %+v", lres.StatusCode, lar)
	}
	if tok, _ := lar.Data["platformToken"].(string); tok == "" {
		t.Fatalf("新密码登录应签发 platformToken: %+v", lar.Data)
	}

	// 反向:错误密码必须被拒(credential_invalid),防「改密后任意密码放行」回退。
	ob, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "loginMethod": "password", "loginAccount": phone, "credential": "WrongPass789",
	})
	ores, oar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", ob, 0, false, false)
	if ores.StatusCode == 200 || oar.Success || oar.Reason != "credential_invalid" {
		t.Fatalf("错误密码应被拒(credential_invalid),得到 %d / %+v", ores.StatusCode, oar)
	}
}
