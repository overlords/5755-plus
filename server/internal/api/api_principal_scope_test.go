package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"m5755/server/internal/signature"
)

// signWith 用指定 keyId/secret 对请求签名并发送,返回响应与解析后的 ApiResult。
// query 用 req.URL.RawQuery 保证与服务端 canonical 一致。
func signWith(t *testing.T, base, keyID, secret, method, path, query string) (*http.Response, apiResp) {
	t.Helper()
	url := base + path
	if query != "" {
		url += "?" + query
	}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Unix()
	for k, v := range signature.Sign(secret, keyID, method, path, req.URL.RawQuery, nil, ts) {
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
	return resp, ar
}

// TestPrincipalScope_ServerKeyAllowedOnLoginCheck 验证 #86 / ADR-0016:
// 游戏服务端 serverKey 签名调登录态校验 GET /api/sdk/v2/subaccount-sessions →
// 验签 + 授权双双通过(非 403、非 signature_invalid;业务结果不论)。
func TestPrincipalScope_ServerKeyAllowedOnLoginCheck(t *testing.T) {
	srv, _ := setup(t)
	query := "gameId=" + seedGame + "&account=does-not-matter"
	resp, ar := signWith(t, srv.URL, serverKeyID, serverSecret,
		"GET", "/api/sdk/v2/subaccount-sessions", query)

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("serverKey 调登录态校验应被授权放行,得到 403;reason=%q", ar.Reason)
	}
	if ar.Reason == "signature_invalid" {
		t.Fatalf("serverKey 签名应验签通过、不应 signature_invalid;reason=%q", ar.Reason)
	}
	if ar.Reason == "principal_not_allowed" {
		t.Fatalf("登录态校验端点不应对 serverKey 拒授权;reason=%q", ar.Reason)
	}
}

// TestPrincipalScope_ServerKeyGameMismatch 验证 serverKey↔game 绑定(grill「检查数据结构」第 1 刀):
// 游戏 A 的 serverKey(dev-server-key,game_id=m5755-demo)调 gameId=B 的登录态校验 → 403 +
// reason=principal_not_allowed(serverKey 与被查游戏不符);验签通过(非 signature_invalid)。
func TestPrincipalScope_ServerKeyGameMismatch(t *testing.T) {
	srv, _ := setup(t)
	query := "account=does-not-matter&gameId=some-other-game"
	resp, ar := signWith(t, srv.URL, serverKeyID, serverSecret,
		"GET", "/api/sdk/v2/subaccount-sessions", query)

	if ar.Reason == "signature_invalid" {
		t.Fatalf("serverKey 签名应验签通过、不应 signature_invalid;reason=%q", ar.Reason)
	}
	if resp.StatusCode != http.StatusForbidden || ar.Reason != "principal_not_allowed" {
		t.Fatalf("serverKey 调他游戏登录态校验应 403 + principal_not_allowed;得到 status=%d reason=%q",
			resp.StatusCode, ar.Reason)
	}
}

// TestPrincipalScope_ServerKeyGameMatch 验证绑定相符放行:
// dev-server-key(game_id=m5755-demo)调 gameId=m5755-demo 的登录态校验 →
// 不因 game 绑定被 403(业务结果不论;非 signature_invalid、非 principal_not_allowed)。
func TestPrincipalScope_ServerKeyGameMatch(t *testing.T) {
	srv, _ := setup(t)
	query := "account=does-not-matter&gameId=" + seedGame
	resp, ar := signWith(t, srv.URL, serverKeyID, serverSecret,
		"GET", "/api/sdk/v2/subaccount-sessions", query)

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("serverKey 调本游戏登录态校验不应 403;reason=%q", ar.Reason)
	}
	if ar.Reason == "principal_not_allowed" {
		t.Fatalf("serverKey↔game 相符不应 principal_not_allowed;reason=%q", ar.Reason)
	}
	if ar.Reason == "signature_invalid" {
		t.Fatalf("serverKey 签名应验签通过;reason=%q", ar.Reason)
	}
}

// TestPrincipalScope_ServerKeyForbiddenOnConfig 验证 serverKey 调登录态校验之外的端点被授权拒绝:
// 验签通过(非 signature_invalid)但 403 + reason=principal_not_allowed。
func TestPrincipalScope_ServerKeyForbiddenOnConfig(t *testing.T) {
	srv, _ := setup(t)
	query := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	resp, ar := signWith(t, srv.URL, serverKeyID, serverSecret,
		"GET", "/api/sdk/v2/config", query)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("serverKey 调 config 应 403 授权拒绝,得到 %d;reason=%q", resp.StatusCode, ar.Reason)
	}
	if ar.Reason != "principal_not_allowed" {
		t.Fatalf("serverKey 越权应 reason=principal_not_allowed(非 signature_invalid);得到 %q", ar.Reason)
	}
}

// TestPrincipalScope_SDKKeyAllowedOnConfig 验证 SDK keyId(principal=sdk)放行所有端点:
// SDK keyId 签名调 config → 通过(非 403、非 signature_invalid)。
func TestPrincipalScope_SDKKeyAllowedOnConfig(t *testing.T) {
	srv, _ := setup(t)
	query := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	resp, ar := signWith(t, srv.URL, seedKeyID, seedSecret,
		"GET", "/api/sdk/v2/config", query)

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("SDK keyId 调 config 不应 403;reason=%q", ar.Reason)
	}
	if ar.Reason == "principal_not_allowed" {
		t.Fatalf("SDK keyId 不应被 principal 作用域拒绝;reason=%q", ar.Reason)
	}
	if ar.Reason == "signature_invalid" {
		t.Fatalf("SDK keyId 签名应验签通过;reason=%q", ar.Reason)
	}
}
