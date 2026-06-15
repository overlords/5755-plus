// Command smoke-alipay 对 sdk-dev 走一遍 SDK 网关面到收银台预下单,验证支付宝沙箱渠道端到端:
// config(验签通) → 密码登录 → 列/建游戏小号 → 小号登录 → 下单 → /pay/begin method=alipay
// → 断言 wap URL 指向沙箱网关。复用 internal/signature 构造 HMAC 头。仅联调用。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"m5755/server/internal/signature"
)

const (
	base                = "https://sdk-dev.xingninghuyu.com"
	keyID               = "dev-test-key"
	secret              = "m5755-dev-public-test-secret-v1"
	gameID              = "m5755-demo"
	headerPlatformToken = "X-M5755-Platform-Token"
)

func signed(method, path, rawQuery string, body map[string]any, extra map[string]string) map[string]any {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	ts := time.Now().Unix()
	hdrs := signature.Sign(secret, keyID, method, path, rawQuery, bodyBytes, ts)
	url := base + path
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req, _ := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	for k, v := range hdrs {
		req.Header.Set(k, v)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return do(req, method, path)
}

func plain(method, path string, body map[string]any) map[string]any {
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, base+path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	return do(req, method, path)
}

func do(req *http.Request, method, path string) map[string]any {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  请求失败 %s %s: %v\n", method, path, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(rb, &out)
	fmt.Printf("  %s %s → %d  %s\n", method, path, resp.StatusCode, truncate(string(rb), 360))
	return out
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
func dataOf(m map[string]any) map[string]any {
	if d, ok := m["data"].(map[string]any); ok {
		return d
	}
	return map[string]any{}
}
func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }

func main() {
	fmt.Println("[1] GET /config")
	signed("GET", "/api/sdk/v2/config", "gameId="+gameID, nil, nil)

	fmt.Println("[2] POST /account-sessions (密码登录)")
	login := dataOf(signed("POST", "/api/sdk/v2/account-sessions", "", map[string]any{
		"gameId": gameID, "loginMethod": "password", "loginAccount": "13900000000", "credential": "Test1234",
	}, nil))
	paID, ptoken := str(login, "platformAccountId"), str(login, "platformToken")
	if paID == "" || ptoken == "" {
		fmt.Println("    未拿到主账户会话。停。")
		os.Exit(1)
	}
	pt := map[string]string{headerPlatformToken: ptoken}

	fmt.Println("[3] GET /subaccounts (列游戏小号)")
	list := dataOf(signed("GET", "/api/sdk/v2/subaccounts", "gameId="+gameID+"&platformAccountId="+paID, nil, pt))
	account := str(list, "defaultAccount")
	if account == "" {
		if subs, ok := list["subaccounts"].([]any); ok && len(subs) > 0 {
			if s0, ok := subs[0].(map[string]any); ok {
				account = str(s0, "account")
			}
		}
	}
	if account == "" {
		fmt.Println("[3b] 无小号,POST /subaccounts 创建")
		created := dataOf(signed("POST", "/api/sdk/v2/subaccounts", "", map[string]any{
			"gameId": gameID, "platformAccountId": paID, "platformToken": ptoken,
		}, nil))
		account = str(created, "account")
	}
	fmt.Printf("    account=%q\n", account)
	if account == "" {
		fmt.Println("    未拿到小号 account。停。")
		os.Exit(1)
	}

	fmt.Println("[4] POST /subaccount-sessions (小号登录拿 token)")
	slog := dataOf(signed("POST", "/api/sdk/v2/subaccount-sessions", "", map[string]any{
		"gameId": gameID, "platformAccountId": paID, "platformToken": ptoken, "account": account,
	}, nil))
	stoken := str(slog, "token")
	if str(slog, "account") != "" {
		account = str(slog, "account")
	}
	fmt.Printf("    account=%q token_len=%d\n", account, len(stoken))
	if stoken == "" {
		fmt.Println("    小号登录未拿到 token(可能实名/防沉迷门禁,见上响应)。停。")
		os.Exit(1)
	}

	fmt.Println("[5] POST /orders (下单)")
	order := dataOf(signed("POST", "/api/sdk/v2/orders", "", map[string]any{
		"gameId": gameID, "account": account, "token": stoken,
		"amount": 0.06, "cpOrderId": fmt.Sprintf("cp_smoke_%d", time.Now().UnixNano()),
		"commodity": "沙箱联调 6 分", "serverId": "s1", "serverName": "星河一区",
		"roleId": "r1", "roleName": "联调角色", "roleLevel": "1",
	}, nil))
	pid := str(order, "platformOrderId")
	fmt.Printf("    platformOrderId=%q\n", pid)
	if pid == "" {
		fmt.Println("    未拿到 platformOrderId。停。")
		os.Exit(1)
	}

	fmt.Println("[6] POST /pay/begin method=alipay (公开端点)")
	begin := dataOf(plain("POST", "/pay/begin", map[string]any{"orderId": pid, "method": "alipay"}))
	wap := str(begin, "redirectUrl")
	fmt.Printf("    kind=%v\n    wapURL=%s\n", begin["kind"], wap)

	fmt.Println("\n=== 断言 ===")
	if wap != "" && bytes.Contains([]byte(wap), []byte("openapi-sandbox.dl.alipaydev.com")) &&
		bytes.Contains([]byte(wap), []byte("method=alipay.trade.wap.pay")) &&
		bytes.Contains([]byte(wap), []byte("sign=")) {
		fmt.Println("✅ wap URL 指向沙箱网关 + wap.pay 方法 + 签名 —— 支付宝沙箱渠道端到端就绪")
	} else {
		fmt.Println("❌ wap URL 不符预期")
		os.Exit(1)
	}
}
