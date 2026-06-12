package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"m5755/server/internal/signature"
)

// loginNewUser 走 sms→login 全链,返回 (phone, platformAccountId, platformToken, firstAccount)。
func loginNewUser(t *testing.T, srv *httptest.Server) (string, string, string, string) {
	t.Helper()
	phone := randomPhone()
	body, _ := json.Marshal(map[string]string{"gameId": seedGame, "loginAccount": phone})
	_, smsAr := doSigned(t, srv.URL, "POST", "/api/sdk/v2/sms-codes", "", body, 0, false, false)
	devCode, _ := smsAr.Data["devCode"].(string)
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "loginMethod": "sms", "loginAccount": phone,
		"credential": devCode, "channelId": "default", "channelSource": "manifest",
	})
	_, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", lb, 0, false, false)
	if !ar.Success {
		t.Fatalf("夹具登录失败: %s", ar.Message)
	}
	paID, _ := ar.Data["platformAccountId"].(string)
	token, _ := ar.Data["platformToken"].(string)
	ge, _ := ar.Data["gameEntry"].(map[string]interface{})
	created, _ := ge["createdSubaccount"].(map[string]interface{})
	first, _ := created["account"].(string)
	return phone, paID, token, first
}

// doSignedH 同 doSigned 但带额外请求头(凭据头不参与 canonical,04 §1.4)。
func doSignedH(t *testing.T, base, method, path, query string, body []byte, headers map[string]string) apiResp {
	t.Helper()
	url := base + path
	if query != "" {
		url += "?" + query
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ts := time.Now().Unix()
	for k, v := range signature.Sign(seedSecret, seedKeyID, method, path, req.URL.RawQuery, body, ts) {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var ar apiResp
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	_ = json.Unmarshal(b, &ar)
	return ar
}

// submitRealName 夹具:为账户提交成年实名。
func submitRealName(t *testing.T, srv *httptest.Server, paID, token string) {
	t.Helper()
	rb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
		"realName": "测试玩家", "idNumber": "11010119900101001X",
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/real-name", "", rb, nil)
	if !ar.Success {
		t.Fatalf("夹具实名失败: %s", ar.Message)
	}
}

// ===== #11 账户有效检查 + kick =====

func TestAccountCheckValidThenKick(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)

	q := fmt.Sprintf("gameId=%s&platformAccountId=%s", seedGame, paID)
	ar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/account-sessions", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	if !ar.Success || ar.Data["valid"] != true {
		t.Fatalf("有效会话应 valid=true: %+v", ar)
	}

	kb, _ := json.Marshal(map[string]string{"gameId": seedGame, "platformAccountId": paID})
	kar := doSignedH(t, srv.URL, "POST", "/internal/dev-control/kick", "", kb, nil)
	if !kar.Success {
		t.Fatalf("kick 失败: %s", kar.Message)
	}

	ar2 := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/account-sessions", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	if !ar2.Success || ar2.Data["valid"] != false || ar2.Reason != "platform_account_invalid" {
		t.Fatalf("kick 后应 success:true+valid=false+platform_account_invalid: %+v", ar2)
	}

	// reset 不恢复已吊销会话(踢号不可逆)
	rb, _ := json.Marshal(map[string]string{"gameId": seedGame})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/reset", "", rb, nil)
	ar3 := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/account-sessions", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	if ar3.Data["valid"] != false {
		t.Fatalf("reset 不应恢复被踢会话")
	}
}

func TestAccountCheckWrongGameTriple(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)
	q := fmt.Sprintf("gameId=%s&platformAccountId=%s", "other-game", paID)
	ar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/account-sessions", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	if !ar.Success || ar.Data["valid"] != false {
		t.Fatalf("gameId 不匹配应 valid=false(三元组): %+v", ar)
	}
}

// ===== #12 实名 + anti-addiction =====

func TestRealNameFlow(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)
	hdr := map[string]string{"X-M5755-Platform-Token": token}
	q := fmt.Sprintf("gameId=%s&platformAccountId=%s", seedGame, paID)

	// 未实名:verified=false,双门禁阻断
	ar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/real-name", q, nil, hdr)
	if ar.Data["verified"] != false || ar.Data["antiAddictionEntryBlocked"] != true || ar.Data["antiAddictionPaymentBlocked"] != true {
		t.Fatalf("未实名门禁推导错误: %+v", ar.Data)
	}

	// 成年实名提交
	rb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
		"realName": "张三", "idNumber": "11010119900101001X",
	})
	ar2 := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/real-name", "", rb, nil)
	if !ar2.Success || ar2.Data["verified"] != true || ar2.Data["adult"] != true || ar2.Data["antiAddictionEntryBlocked"] != false {
		t.Fatalf("成年实名后应放行进入: %+v", ar2.Data)
	}

	// 幂等锁定:再提交不同身份证,值不变(仍成年)
	rb2, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
		"realName": "李四", "idNumber": "110101" + "20150101" + "0011",
	})
	ar3 := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/real-name", "", rb2, nil)
	if !ar3.Success || ar3.Data["adult"] != true {
		t.Fatalf("已实名重复提交应幂等成功不改值: %+v", ar3.Data)
	}
}

func TestRealNameMinorAndInvalid(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)
	// 非法格式
	bad, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
		"realName": "张三", "idNumber": "123",
	})
	arBad := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/real-name", "", bad, nil)
	if arBad.Success || arBad.Reason != "param_invalid" {
		t.Fatalf("非法身份证应 param_invalid: %+v", arBad)
	}
	// 未成年(2015 年生):放行进入、阻断支付
	minor, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
		"realName": "小明", "idNumber": "110101201501010011",
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/real-name", "", minor, nil)
	if !ar.Success || ar.Data["adult"] != false || ar.Data["antiAddictionEntryBlocked"] != false || ar.Data["antiAddictionPaymentBlocked"] != true {
		t.Fatalf("未成年门禁推导错误: %+v", ar.Data)
	}
}

func TestAntiAddictionInjectionOverrides(t *testing.T) {
	srv, st := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)
	submitRealName(t, srv, paID, token)
	t.Cleanup(func() { _ = st.ClearGameInjections(t.Context(), seedGame) })

	ib, _ := json.Marshal(map[string]interface{}{
		"gameId": seedGame, "platformAccountId": paID, "entryBlocked": true, "paymentBlocked": true,
	})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/anti-addiction", "", ib, nil)

	q := fmt.Sprintf("gameId=%s&platformAccountId=%s", seedGame, paID)
	ar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/real-name", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	if ar.Data["antiAddictionEntryBlocked"] != true {
		t.Fatalf("注入应覆盖门禁: %+v", ar.Data)
	}

	rb, _ := json.Marshal(map[string]string{"gameId": seedGame})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/reset", "", rb, nil)
	ar2 := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/real-name", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	if ar2.Data["antiAddictionEntryBlocked"] != false {
		t.Fatalf("reset 后应恢复真实判定: %+v", ar2.Data)
	}
}

// ===== #13 小号 =====

func TestSubaccountLimitAndNaming(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)
	cb, _ := json.Marshal(map[string]string{"gameId": seedGame, "platformAccountId": paID, "platformToken": token})

	// 首个已建档,补到 10 个
	for i := 0; i < 9; i++ {
		ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccounts", "", cb, nil)
		if !ar.Success {
			t.Fatalf("第 %d 个创建失败: %s", i+2, ar.Message)
		}
		if ar.Data["isDefault"] != false {
			t.Fatalf("新增小号应恒非默认")
		}
	}
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccounts", "", cb, nil)
	if ar.Success || ar.Reason != "subaccount_limit_reached" {
		t.Fatalf("第 11 个应 subaccount_limit_reached: %+v", ar)
	}

	q := fmt.Sprintf("gameId=%s&platformAccountId=%s", seedGame, paID)
	lar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/subaccounts", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	subs, _ := lar.Data["subaccounts"].([]interface{})
	if len(subs) != 10 {
		t.Fatalf("列表应 10 个,得 %d", len(subs))
	}
	last, _ := subs[9].(map[string]interface{})
	if last["displayName"] != "小号10" {
		t.Fatalf("命名应递增到 小号10,得 %v", last["displayName"])
	}
}

func TestSubaccountCreateIgnoresIsDefault(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)
	cb, _ := json.Marshal(map[string]interface{}{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "isDefault": true,
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccounts", "", cb, nil)
	if !ar.Success || ar.Data["isDefault"] != false {
		t.Fatalf("isDefault 入参应被忽略: %+v", ar.Data)
	}
}

func TestSetDefaultIdempotentAndExclusive(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, first := loginNewUser(t, srv)
	cb, _ := json.Marshal(map[string]string{"gameId": seedGame, "platformAccountId": paID, "platformToken": token})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccounts", "", cb, nil)
	second, _ := ar.Data["account"].(string)

	setDefault := func(acc string) apiResp {
		db, _ := json.Marshal(map[string]string{
			"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "account": acc,
		})
		return doSignedH(t, srv.URL, "PUT", "/api/sdk/v2/subaccounts/default", "", db, nil)
	}
	if r := setDefault(first); !r.Success {
		t.Fatalf("设默认失败: %s", r.Message)
	}
	if r := setDefault(first); !r.Success {
		t.Fatalf("幂等重设应成功")
	}
	if r := setDefault(second); !r.Success {
		t.Fatalf("切换默认失败")
	}
	q := fmt.Sprintf("gameId=%s&platformAccountId=%s", seedGame, paID)
	lar := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/subaccounts", q, nil, map[string]string{"X-M5755-Platform-Token": token})
	if lar.Data["defaultAccount"] != second {
		t.Fatalf("互斥:默认应为 %s,得 %v", second, lar.Data["defaultAccount"])
	}
	if r := setDefault("sub_nonexistent"); r.Success || r.Reason != "subaccount_invalid" {
		t.Fatalf("不存在小号应 subaccount_invalid: %+v", r)
	}
}

// ===== #14 小号会话 + 分流矩阵 =====

func TestSubaccountLoginAndCheck(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, first := loginNewUser(t, srv)
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "account": first,
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	if !ar.Success || ar.Data["token"] == nil {
		t.Fatalf("小号登录应签发 token: %+v", ar)
	}
	subToken, _ := ar.Data["token"].(string)

	q := fmt.Sprintf("account=%s&gameId=%s", first, seedGame)
	car := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/subaccount-sessions", q, nil, map[string]string{"X-M5755-Token": subToken})
	if !car.Success || car.Data["valid"] != true {
		t.Fatalf("登录态校验应 valid=true: %+v", car)
	}
}

func TestSubaccountLoginRequiresAccount(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, _ := loginNewUser(t, srv)
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token,
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	if ar.Success || ar.Reason != "param_invalid" {
		t.Fatalf("省略 account 应 param_invalid(无默认兜底): %+v", ar)
	}
}

func TestInvalidationRouting(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, first := loginNewUser(t, srv)
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "account": first,
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	subToken, _ := ar.Data["token"].(string)

	// 小号失效 → subaccount_invalid(登录与校验两路)
	ib, _ := json.Marshal(map[string]string{"gameId": seedGame, "account": first})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/invalidate-subaccount", "", ib, nil)

	ar2 := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	if ar2.Success || ar2.Reason != "subaccount_invalid" {
		t.Fatalf("小号失效后登录应 subaccount_invalid: %+v", ar2)
	}
	q := fmt.Sprintf("account=%s&gameId=%s", first, seedGame)
	car := doSignedH(t, srv.URL, "GET", "/api/sdk/v2/subaccount-sessions", q, nil, map[string]string{"X-M5755-Token": subToken})
	if !car.Success || car.Data["valid"] != false || car.Reason != "subaccount_invalid" {
		t.Fatalf("已签发令牌校验应 valid=false+subaccount_invalid: %+v", car)
	}
}

func TestKickRoutesToPlatformAccountInvalid(t *testing.T) {
	srv, _ := setup(t)
	_, paID, token, first := loginNewUser(t, srv)
	kb, _ := json.Marshal(map[string]string{"gameId": seedGame, "platformAccountId": paID})
	doSignedH(t, srv.URL, "POST", "/internal/dev-control/kick", "", kb, nil)

	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "account": first,
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	if ar.Success || ar.Reason != "platform_account_invalid" {
		t.Fatalf("踢号后小号登录应 platform_account_invalid(与小号失效分流不混): %+v", ar)
	}
}

func TestSubaccountOwnershipRouting(t *testing.T) {
	srv, _ := setup(t)
	_, _, _, otherFirst := loginNewUser(t, srv) // 他人小号
	_, paID, token, _ := loginNewUser(t, srv)
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "platformAccountId": paID, "platformToken": token, "account": otherFirst,
	})
	ar := doSignedH(t, srv.URL, "POST", "/api/sdk/v2/subaccount-sessions", "", lb, nil)
	if ar.Success || ar.Reason != "subaccount_invalid" {
		t.Fatalf("他人小号应 subaccount_invalid: %+v", ar)
	}
}
