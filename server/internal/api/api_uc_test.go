package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
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
