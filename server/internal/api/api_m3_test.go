package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"m5755/server/internal/domain"
)

// loginToSubaccount 走完整链拿到小号 account+token(成年实名)。
func loginToSubaccount(t *testing.T, srv *httptest.Server) (account, subToken, paID string) {
	t.Helper()
	_, paID, token, first := loginNewUser(t, srv)
	submitRealName(t, srv, paID, token)
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "account": first,
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	if !ar.Success {
		t.Fatalf("小号登录失败: %s", ar.Message)
	}
	return first, ar.Data["token"].(string), paID
}

func roleBody(account, token string, override map[string]string) []byte {
	m := map[string]string{
		"gameId": seedGame, "account": account, "token": token,
		"serverId": "s1", "serverName": "星河一区", "roleId": "role_1", "roleName": "云起",
		"roleLevel": "68", "roleCe": "128000", "roleStage": "12-6", "roleRechargeAmount": "328.00", "roleGuild": "无",
	}
	for k, v := range override {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

// ===== #21 角色上报 =====

func TestRoleReportOK(t *testing.T) {
	srv, _ := setup(t)
	account, token, _ := loginToSubaccount(t, srv)
	ar := doSignedH(t, srv.URL, "PUT", "/api/sdk/v2/roles", "", roleBody(account, token, nil), nil)
	if !ar.Success || ar.Data["reported"] != true {
		t.Fatalf("角色上报应成功: %+v", ar)
	}
	// upsert:再报更新等级
	ar2 := doSignedH(t, srv.URL, "PUT", "/api/sdk/v2/roles", "", roleBody(account, token, map[string]string{"roleLevel": "70"}), nil)
	if !ar2.Success {
		t.Fatalf("重复上报应 upsert 成功")
	}
}

func TestRoleReportValidation(t *testing.T) {
	srv, _ := setup(t)
	account, token, _ := loginToSubaccount(t, srv)
	// roleId=-1 拒绝
	if ar := doSignedH(t, srv.URL, "PUT", "/api/sdk/v2/roles", "", roleBody(account, token, map[string]string{"roleId": "-1"}), nil); ar.Success || ar.Reason != "param_invalid" {
		t.Fatalf("roleId=-1 应拒绝: %+v", ar)
	}
	// 充值额=-1 接受
	if ar := doSignedH(t, srv.URL, "PUT", "/api/sdk/v2/roles", "", roleBody(account, token, map[string]string{"roleRechargeAmount": "-1"}), nil); !ar.Success {
		t.Fatalf("充值额=-1 应接受: %+v", ar)
	}
	// 充值额非两位小数拒绝
	if ar := doSignedH(t, srv.URL, "PUT", "/api/sdk/v2/roles", "", roleBody(account, token, map[string]string{"roleRechargeAmount": "328.5"}), nil); ar.Success {
		t.Fatalf("328.5 应拒绝")
	}
	// 字段缺失
	if ar := doSignedH(t, srv.URL, "PUT", "/api/sdk/v2/roles", "", roleBody(account, token, map[string]string{"roleName": ""}), nil); ar.Success {
		t.Fatalf("roleName 空应拒绝")
	}
}

// ===== #22 支付创建 + 门禁复核 =====

func orderBody(account, token string, override map[string]interface{}) []byte {
	m := map[string]interface{}{
		"gameId": seedGame, "account": account, "token": token, "amount": "328.00",
		"cpOrderId": "cp_" + account, "commodity": "648 元宝", "serverId": "s1", "serverName": "星河一区",
		"roleId": "role_1", "roleName": "云起", "roleLevel": "68",
	}
	for k, v := range override {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

func TestOrderCreateOK(t *testing.T) {
	srv, _ := setup(t)
	account, token, _ := loginToSubaccount(t, srv)
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(account, token, nil), nil)
	if !ar.Success || ar.Data["orderId"] == nil || ar.Data["paymentUrl"] == nil {
		t.Fatalf("支付创建应返回订单号+paymentUrl: %+v", ar)
	}
	pu, _ := ar.Data["paymentUrl"].(string)
	if pu == "" {
		t.Fatalf("paymentUrl 不应为空")
	}
}

func TestOrderValidation(t *testing.T) {
	srv, _ := setup(t)
	account, token, _ := loginToSubaccount(t, srv)
	for _, tc := range []map[string]interface{}{
		{"amount": "0.00"},      // 值非正
		{"amount": "328"},       // 非两位小数(无小数点)
		{"amount": "328.5"},     // 非两位小数(一位)
		{"amount": "1000000000.00"}, // 超上限(>= 1e9)
		{"amount": 328.0},       // 数字而非字符串(bind 失败)
		{"cpOrderId": ""},
		{"commodity": ""},
		{"serverId": ""},
	} {
		if ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(account, token, tc), nil); ar.Success || ar.Reason != "order_invalid" {
			t.Fatalf("非法订单 %v 应 order_invalid: %+v", tc, ar)
		}
	}
}

func TestOrderPaymentGate(t *testing.T) {
	skipIfProd(t)
	srv, st := setup(t)
	account, token, paID := loginToSubaccount(t, srv)
	t.Cleanup(func() { _ = st.ClearGameInjections(t.Context(), seedGame) })
	ib, _ := json.Marshal(map[string]interface{}{"gameId": seedGame, "platformAccountId": paID, "paymentBlocked": true})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/anti-addiction", "", ib, nil)
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(account, token, nil), nil)
	if ar.Success || ar.Reason != "anti_addiction_payment_blocked" {
		t.Fatalf("支付门禁应拦截创建: %+v", ar)
	}
}

func TestOrderRealNameRequired(t *testing.T) {
	srv, _ := setup(t)
	// 不实名直接登录小号(跳过 submitRealName)
	_, paID, token, first := loginNewUser(t, srv)
	lb, _ := json.Marshal(map[string]string{"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "account": first})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	subToken := ar.Data["token"].(string)
	or := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(first, subToken, nil), nil)
	if or.Success || or.Reason != "real_name_required" {
		t.Fatalf("未实名应 real_name_required: %+v", or)
	}
}

// ===== #23 充值回调出站(本地接收端) =====

type callbackReceiver struct {
	mu     sync.Mutex
	hits   []map[string]string
	srv    *httptest.Server
	ackBad bool
}

func newReceiver() *callbackReceiver {
	r := &callbackReceiver{}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		var m map[string]string
		_ = json.Unmarshal(b, &m)
		r.mu.Lock()
		r.hits = append(r.hits, m)
		bad := r.ackBad
		r.mu.Unlock()
		if bad {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"msg":"success"}`))
	}))
	return r
}

func (r *callbackReceiver) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.hits)
}

func TestCallbackDispatchAndIdempotentRepush(t *testing.T) {
	skipIfProd(t)
	srv, st := setup(t)
	rec := newReceiver()
	defer rec.srv.Close()
	if err := st.SetCallbackURL(t.Context(), seedGame, rec.srv.URL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.SetCallbackURL(t.Context(), seedGame, "") })

	account, token, _ := loginToSubaccount(t, srv)
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(account, token, nil), nil)
	orderID := ar.Data["orderId"].(string)

	// complete-payment 成功 → 接收端收到回调
	cb, _ := json.Marshal(map[string]string{"gameId": seedGame, "orderId": orderID, "mode": "成功"})
	if r := doSignedH(t, srv.URL, "POST", "/internal/dev-control/complete-payment", "", cb, nil); !r.Success {
		t.Fatalf("complete-payment 失败: %s", r.Message)
	}
	if rec.count() != 1 {
		t.Fatalf("接收端应收到 1 次回调,得 %d", rec.count())
	}
	got := rec.hits[0]
	// 回调体定形(ADR-0016):orderId/payAmount/serverKeyId,payAmount 恒等 amount。
	if got["account"] != account || got["orderId"] != orderID || got["cpOrderId"] != "cp_"+account ||
		got["payAmount"] != got["amount"] || got["serverKeyId"] != "dev-server-key" {
		t.Fatalf("回调字段不符定形: %+v", got)
	}
	if !domain.VerifyCallbackSign(got, "m5755-dev-callback-secret-v1") {
		t.Fatalf("回调签名校验失败")
	}

	// 订单查询:callbackStatus 反映已确认
	q := fmt.Sprintf("gameId=%s&account=%s", seedGame, account)
	qr := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/orders/"+orderID, q, nil, map[string]string{"X-M5755-Token": token})
	if qr.Data["paymentStatus"] != "已支付" || qr.Data["callbackStatus"] != "已确认" {
		t.Fatalf("订单状态应已支付/已确认: %+v", qr.Data)
	}
}

// TestRedeliverPendingCallbacksHealsFailedDelivery 验证 blocker 修复:出站投递失败的订单
// 不靠渠道重推、由平台侧巡检重投自愈。游戏服务端先 500(投递失败)→ 恢复 → 巡检 → 已确认。
func TestRedeliverPendingCallbacksHealsFailedDelivery(t *testing.T) {
	skipIfProd(t)
	srv, st := setup(t)
	rec := newReceiver()
	defer rec.srv.Close()
	rec.mu.Lock()
	rec.ackBad = true // 游戏服务端先抖动:回 500 → 投递失败
	rec.mu.Unlock()
	if err := st.SetCallbackURL(t.Context(), seedGame, rec.srv.URL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.SetCallbackURL(t.Context(), seedGame, "") })

	account, token, _ := loginToSubaccount(t, srv)
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(account, token, nil), nil)
	orderID := ar.Data["orderId"].(string)

	// complete-payment:回调投递失败 → 订单卡在 已支付/投递失败(漏发前置)
	cb, _ := json.Marshal(map[string]string{"gameId": seedGame, "orderId": orderID, "mode": "成功"})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/complete-payment", "", cb, nil)
	q := fmt.Sprintf("gameId=%s&account=%s", seedGame, account)
	if qr := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/orders/"+orderID, q, nil, map[string]string{"X-M5755-Token": token}); qr.Data["callbackStatus"] != "投递失败" {
		t.Fatalf("漏发前置不成立,应 投递失败: %+v", qr.Data)
	}

	// 游戏服务端恢复 → 平台侧巡检重投(不依赖渠道重推)→ 自愈为已确认
	rec.mu.Lock()
	rec.ackBad = false
	rec.mu.Unlock()
	attempted, confirmed := domain.New(st).RedeliverPendingCallbacks(t.Context())
	if attempted < 1 || confirmed < 1 {
		t.Fatalf("重投应尝试并确认 ≥1 笔,得 attempted=%d confirmed=%d", attempted, confirmed)
	}
	if qr2 := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/orders/"+orderID, q, nil, map[string]string{"X-M5755-Token": token}); qr2.Data["callbackStatus"] != "已确认" {
		t.Fatalf("重投后应自愈为 已确认: %+v", qr2.Data)
	}
}

func TestCallbackTimeoutRepush(t *testing.T) {
	skipIfProd(t)
	srv, st := setup(t)
	rec := newReceiver()
	defer rec.srv.Close()
	_ = st.SetCallbackURL(t.Context(), seedGame, rec.srv.URL)
	t.Cleanup(func() { _ = st.SetCallbackURL(t.Context(), seedGame, "") })
	account, token, _ := loginToSubaccount(t, srv)
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/orders", "", orderBody(account, token, nil), nil)
	orderID := ar.Data["orderId"].(string)
	cb, _ := json.Marshal(map[string]string{"gameId": seedGame, "orderId": orderID, "mode": "超时"})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/complete-payment", "", cb, nil)
	if rec.count() < 2 {
		t.Fatalf("超时模式应重复推送(≥2 次),得 %d", rec.count())
	}
	if rec.hits[0]["sign"] != rec.hits[1]["sign"] {
		t.Fatalf("重复推送同笔字段应一致(签名相同)")
	}
}

// ===== #24 故障注入 =====

func TestFaultMalformedAndAutoExpire(t *testing.T) {
	skipIfProd(t)
	srv, _ := setup(t)
	t.Cleanup(func() {
		rb, _ := json.Marshal(map[string]string{"gameId": seedGame})
		doSignedH(t, srv.URL, "POST", "/internal/dev-control/reset", "", rb, nil)
	})
	fb, _ := json.Marshal(map[string]interface{}{
		"gameId": seedGame, "endpoint": "/api/sdk/v2/config", "type": "malformed", "times": 1,
	})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/fault", "", fb, nil)

	q := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	// 第一次:畸形响应 → 解析失败 → ar 非成功(success 默认 false)
	ar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, nil)
	if ar.Success {
		t.Fatalf("注入畸形响应后首次不应成功")
	}
	// 第二次:自动失效 → 正常
	ar2 := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, nil)
	if !ar2.Success {
		t.Fatalf("times=1 应自动失效,第二次恢复正常: %+v", ar2)
	}
}

func TestFaultHttp500ScopedByGame(t *testing.T) {
	srv, _ := setup(t)
	t.Cleanup(func() {
		rb, _ := json.Marshal(map[string]string{"gameId": seedGame})
		doSignedH(t, srv.URL, "POST", "/internal/dev-control/reset", "", rb, nil)
	})
	// 注入到别的游戏端点,不应影响 seedGame
	fb, _ := json.Marshal(map[string]interface{}{"gameId": "other", "endpoint": "/api/sdk/v2/config", "type": "http500", "times": 3})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/fault", "", fb, nil)
	q := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	if ar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, nil); !ar.Success {
		t.Fatalf("故障作用域应限定游戏,seedGame 不受影响: %+v", ar)
	}
}

// ===== #25 密码登录 + 设备验证 =====

func passwordLogin(t *testing.T, srv *httptest.Server, password, deviceID, verifyCode string) apiResp {
	t.Helper()
	body := map[string]string{
		"gameId": seedGame, "loginMethod": "password", "loginAccount": "13900000000",
		"credential": password, "channelId": "default", "channelSource": "manifest",
	}
	if deviceID != "" {
		body["deviceId"] = deviceID
	}
	if verifyCode != "" {
		body["deviceVerifyCode"] = verifyCode
	}
	b, _ := json.Marshal(body)
	return doSignedH(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", b, nil)
}

func TestPasswordLoginAndDeviceVerification(t *testing.T) {
	srv, st := setup(t)
	// 设备验证默认关:本用例锁定「开启」路径,故先把该游戏开关置 true。
	if err := st.SetDeviceVerificationEnabled(t.Context(), seedGame, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.SetDeviceVerificationEnabled(context.Background(), seedGame, false) })
	// 测试不经 main,需手动种密码账户(同 dev 启动种子)
	hash, _ := domain.HashPassword("Test1234")
	if err := st.EnsureDevPasswordAccount(t.Context(), "13900000000", "密码测试账户", hash); err != nil {
		t.Fatal(err)
	}
	dev := "dev-" + randomPhone()

	// 错误密码
	if ar := passwordLogin(t, srv, "wrong", dev, ""); ar.Success || ar.Reason != "credential_invalid" {
		t.Fatalf("错误密码应 credential_invalid: %+v", ar)
	}
	// 新设备首次:需设备验证
	if ar := passwordLogin(t, srv, "Test1234", dev, ""); ar.Success || ar.Reason != "device_verification_required" {
		t.Fatalf("新设备应 device_verification_required: %+v", ar)
	}
	// 取设备验证码(发往账户绑定手机号)
	sb, _ := json.Marshal(map[string]string{"gameId": seedGame, "loginAccount": "13900000000"})
	smsAr := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/sms-codes", "", sb, nil)
	code := smsAr.Data["devCode"].(string)
	// 带验证码:信任设备并登录成功
	if ar := passwordLogin(t, srv, "Test1234", dev, code); !ar.Success || ar.Data["platformToken"] == nil {
		t.Fatalf("设备验证后应登录成功: %+v", ar)
	}
	// 同设备再次:已信任,无需验证码
	if ar := passwordLogin(t, srv, "Test1234", dev, ""); !ar.Success {
		t.Fatalf("已信任设备应直接登录: %+v", ar)
	}
	// 新设备:仍需验证(逐设备)
	if ar := passwordLogin(t, srv, "Test1234", "dev2-"+randomPhone(), ""); ar.Reason != "device_verification_required" {
		t.Fatalf("另一新设备应再次需验证: %+v", ar)
	}
}

// TestPasswordLoginMissingDeviceIDFailsClosed 锁定 #25 fail-closed:密码正确但 deviceId 为空时
// 必须 400 + reason=param_invalid(缺 deviceId),不得回退到 fail-open「缺则视为已信任」直接放行。
// 防御:攻击者省略 deviceId 绕过设备校验。用 doSigned(返回 *http.Response)以校验 HTTP 状态码。
func TestPasswordLoginMissingDeviceIDFailsClosed(t *testing.T) {
	srv, st := setup(t)
	// fail-closed 仅在设备验证开启时生效;默认关下缺 deviceId 是纯密码登录成功(见
	// TestPasswordLoginDefaultOffNoDeviceIDSucceeds),故本用例先开启开关。
	if err := st.SetDeviceVerificationEnabled(t.Context(), seedGame, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.SetDeviceVerificationEnabled(context.Background(), seedGame, false) })
	hash, _ := domain.HashPassword("Test1234")
	if err := st.EnsureDevPasswordAccount(t.Context(), "13900000000", "密码测试账户", hash); err != nil {
		t.Fatal(err)
	}
	// 正确密码 + 空 deviceId:密码校验通过后撞 deviceId 必填检查 → 400 param_invalid。
	b, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "loginMethod": "password", "loginAccount": "13900000000",
		"credential": "Test1234", "channelId": "default", "channelSource": "manifest",
		// 故意不带 deviceId
	})
	res, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", b, 0, false, false)
	if ar.Success {
		t.Fatalf("缺 deviceId 不应登录成功: %+v", ar)
	}
	if res.StatusCode != 400 || ar.Reason != "param_invalid" {
		t.Fatalf("缺 deviceId 应 400/param_invalid,得到 %d / %+v", res.StatusCode, ar)
	}
}

// TestPasswordLoginDefaultOffNoDeviceIDSucceeds 锁定 #25 默认关语义:demo 游戏
// device_verification_enabled 默认 false → 正确密码 + 不带 deviceId → 密码登录成功
// (200 + 签发 platformToken),整个设备信任块被跳过。这是 v2 版本默认行为的护栏。
func TestPasswordLoginDefaultOffNoDeviceIDSucceeds(t *testing.T) {
	srv, st := setup(t)
	// 不调 SetDeviceVerificationEnabled:依赖 migration 0015 的 default false。
	hash, _ := domain.HashPassword("Test1234")
	if err := st.EnsureDevPasswordAccount(t.Context(), "13900000000", "密码测试账户", hash); err != nil {
		t.Fatal(err)
	}
	// 正确密码 + 故意不带 deviceId:默认关 → 纯密码登录成功。
	b, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "loginMethod": "password", "loginAccount": "13900000000",
		"credential": "Test1234", "channelId": "default", "channelSource": "manifest",
	})
	res, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", b, 0, false, false)
	if res.StatusCode != 200 || !ar.Success {
		t.Fatalf("默认关 + 无 deviceId 应密码登录成功,得到 %d / %+v", res.StatusCode, ar)
	}
	if tok, _ := ar.Data["platformToken"].(string); tok == "" {
		t.Fatalf("默认关密码登录应签发 platformToken: %+v", ar.Data)
	}
}
