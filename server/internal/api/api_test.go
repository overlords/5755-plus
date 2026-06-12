package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"m5755/server/internal/domain"
	"m5755/server/internal/signature"
	"m5755/server/internal/store"
)

const (
	seedGame   = "m5755-demo"
	seedKeyID  = "dev-test-key"
	seedSecret = "m5755-dev-public-test-secret-v1"
)

// setup 连接真实 Postgres(DATABASE_URL),套迁移,返回 httptest server。
func setup(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 DATABASE_URL,跳过 HTTP-seam 测试")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	srv := httptest.NewServer(NewRouter(domain.New(st), st, time.Now))
	t.Cleanup(func() { srv.Close(); st.Close() })
	return srv, st
}

type apiResp struct {
	Success bool                   `json:"success"`
	Code    int                    `json:"code"`
	Reason  string                 `json:"reason"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

// doSigned 发送已签名请求。ts=0 用当前时间。omitSig=true 不带签名头。
func doSigned(t *testing.T, base, method, path, query string, body []byte, ts int64, omitSig bool, tamper bool) (*http.Response, apiResp) {
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
	if !omitSig {
		if ts == 0 {
			ts = time.Now().Unix()
		}
		// query 用 req.URL.RawQuery,保证与服务端一致
		headers := signature.Sign(seedSecret, seedKeyID, method, path, req.URL.RawQuery, body, ts)
		if tamper {
			headers[signature.HeaderSignature] = "deadbeef"
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
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

func randomPhone() string {
	b := make([]byte, 10)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		b[i] = byte('0' + n.Int64())
	}
	return "1" + string(b)
}

func TestHealthz(t *testing.T) {
	srv, _ := setup(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz 期望 200,得 %d", resp.StatusCode)
	}
}

func TestConfigSignedOK(t *testing.T) {
	srv, _ := setup(t)
	q := "gameId=" + seedGame + "&sdkVersion=1.0.0&packageName=com.x&channelId=default&channelSource=manifest"
	resp, ar := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, 0, false, false)
	if resp.StatusCode != 200 || !ar.Success {
		t.Fatalf("期望成功,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
	if ar.Data["protocolVersion"] == nil || ar.Data["configVersion"] == nil {
		t.Fatalf("config 缺少必要字段: %+v", ar.Data)
	}
	if _, ok := ar.Data["accountNickname"]; ok {
		t.Fatalf("config 不应返回 accountNickname")
	}
	if ar.Data["requestId"] == nil {
		t.Fatalf("config 应返回 requestId")
	}
}

func TestConfigMissingSignature(t *testing.T) {
	srv, _ := setup(t)
	q := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	resp, ar := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, 0, true, false)
	if resp.StatusCode != 401 || ar.Reason != "signature_invalid" {
		t.Fatalf("期望 401 signature_invalid,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
}

func TestConfigBadSignature(t *testing.T) {
	srv, _ := setup(t)
	q := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	resp, ar := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, 0, false, true)
	if resp.StatusCode != 401 || ar.Reason != "signature_invalid" {
		t.Fatalf("期望 401 signature_invalid,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
}

func TestConfigTimestampExpired(t *testing.T) {
	srv, _ := setup(t)
	q := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	old := time.Now().Add(-10 * time.Minute).Unix()
	resp, ar := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, old, false, false)
	if resp.StatusCode != 401 || ar.Reason != "timestamp_expired" {
		t.Fatalf("期望 401 timestamp_expired,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
}

func TestConfigMissingGameId(t *testing.T) {
	srv, _ := setup(t)
	resp, ar := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", "sdkVersion=1.0.0", nil, 0, false, false)
	if resp.StatusCode == 200 || ar.Success {
		t.Fatalf("gameId 缺失应失败,得 status=%d", resp.StatusCode)
	}
	if ar.Reason != "param_invalid" {
		t.Fatalf("期望 param_invalid,得 %s", ar.Reason)
	}
}

func TestConfigUnknownGame(t *testing.T) {
	srv, _ := setup(t)
	resp, ar := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", "gameId=nope-game&sdkVersion=1.0.0", nil, 0, false, false)
	if resp.StatusCode != 404 || ar.Success {
		t.Fatalf("未知游戏应 404 失败,得 status=%d", resp.StatusCode)
	}
}

func TestConfigUpdateRequired(t *testing.T) {
	srv, _ := setup(t)
	// seed sdkMinVersion=1.0.0;传 0.9.0 应要求更新
	q := "gameId=" + seedGame + "&sdkVersion=0.9.0"
	_, ar := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, 0, false, false)
	if ar.Data["updateRequired"] != true {
		t.Fatalf("0.9.0 < 1.0.0 应 updateRequired=true,得 %v", ar.Data["updateRequired"])
	}
}

func TestSmsCodesOK(t *testing.T) {
	srv, _ := setup(t)
	phone := randomPhone()
	body, _ := json.Marshal(map[string]string{"gameId": seedGame, "loginAccount": phone})
	resp, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/sms-codes", "", body, 0, false, false)
	if resp.StatusCode != 200 || !ar.Success {
		t.Fatalf("sms-codes 期望成功,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
	if ar.Data["devCode"] == nil {
		t.Fatalf("mock 模式应返回 devCode")
	}
	if ar.Data["loginAccountMasked"] == phone {
		t.Fatalf("不应返回完整手机号")
	}
}

func TestSmsCodesBadPhone(t *testing.T) {
	srv, _ := setup(t)
	body, _ := json.Marshal(map[string]string{"gameId": seedGame, "loginAccount": "abc"})
	resp, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/sms-codes", "", body, 0, false, false)
	if resp.StatusCode == 200 || ar.Reason != "param_invalid" {
		t.Fatalf("非手机号应 param_invalid,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
}

func TestAccountSessionsLoginOK(t *testing.T) {
	srv, _ := setup(t)
	phone := randomPhone()
	// 先请求验证码
	body, _ := json.Marshal(map[string]string{"gameId": seedGame, "loginAccount": phone})
	_, smsAr := doSigned(t, srv.URL, "POST", "/api/sdk/v2/sms-codes", "", body, 0, false, false)
	devCode, _ := smsAr.Data["devCode"].(string)
	if devCode == "" {
		t.Fatal("未取得 devCode")
	}
	// 登录
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "loginMethod": "sms", "loginAccount": phone,
		"credential": devCode, "channelId": "default", "channelSource": "manifest",
	})
	resp, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", lb, 0, false, false)
	if resp.StatusCode != 200 || !ar.Success {
		t.Fatalf("登录应成功,得 status=%d reason=%s msg=%s", resp.StatusCode, ar.Reason, ar.Message)
	}
	if ar.Data["platformToken"] == nil || ar.Data["platformAccountId"] == nil {
		t.Fatalf("登录应返回 platformToken/platformAccountId")
	}
	// 不返回小号 token
	if _, ok := ar.Data["token"]; ok {
		t.Fatalf("账户登录不应返回小号 token")
	}
	// 新用户首个小号建档
	ge, _ := ar.Data["gameEntry"].(map[string]interface{})
	if ge == nil || ge["createdSubaccount"] == nil {
		t.Fatalf("新用户应建档首个小号: %+v", ar.Data["gameEntry"])
	}
}

func TestAccountSessionsInvalidCode(t *testing.T) {
	srv, _ := setup(t)
	phone := randomPhone()
	lb, _ := json.Marshal(map[string]string{
		"gameId": seedGame, "loginMethod": "sms", "loginAccount": phone,
		"credential": "000000", "channelId": "default", "channelSource": "manifest",
	})
	resp, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/account-sessions", "", lb, 0, false, false)
	if resp.StatusCode == 200 || ar.Reason != "sms_code_invalid" {
		t.Fatalf("错误验证码应 sms_code_invalid,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
}

func TestDevControlMaintenanceDrivesConfig(t *testing.T) {
	srv, st := setup(t)
	ctx := context.Background()
	t.Cleanup(func() { _ = st.ClearInjections(ctx, seedGame) })

	// 置维护
	mb, _ := json.Marshal(map[string]interface{}{"gameId": seedGame, "enabled": true, "message": "维护中"})
	resp, ar := doSigned(t, srv.URL, "POST", "/internal/dev-control/maintenance", "", mb, 0, false, false)
	if resp.StatusCode != 200 || !ar.Success {
		t.Fatalf("置维护失败 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}

	// config 反映维护
	q := "gameId=" + seedGame + "&sdkVersion=1.0.0"
	_, cfg := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, 0, false, false)
	m, _ := cfg.Data["maintenance"].(map[string]interface{})
	if m == nil || m["enabled"] != true {
		t.Fatalf("config 应反映维护中: %+v", cfg.Data["maintenance"])
	}

	// state 反映注入
	_, stateAr := doSigned(t, srv.URL, "GET", "/internal/dev-control/state", "gameId="+seedGame, nil, 0, false, false)
	if stateAr.Data["maintenanceEnabled"] != true {
		t.Fatalf("state 应反映维护注入: %+v", stateAr.Data)
	}

	// reset 清除
	rb, _ := json.Marshal(map[string]string{"gameId": seedGame})
	doSigned(t, srv.URL, "POST", "/internal/dev-control/reset", "", rb, 0, false, false)
	_, cfg2 := doSigned(t, srv.URL, "GET", "/api/sdk/v2/config", q, nil, 0, false, false)
	m2, _ := cfg2.Data["maintenance"].(map[string]interface{})
	if m2 != nil && m2["enabled"] == true {
		t.Fatalf("reset 后 config 不应再维护")
	}
}

func TestDevControlNeedsSignature(t *testing.T) {
	srv, _ := setup(t)
	rb, _ := json.Marshal(map[string]string{"gameId": seedGame})
	resp, ar := doSigned(t, srv.URL, "POST", "/internal/dev-control/reset", "", rb, 0, true, false)
	if resp.StatusCode != 401 || ar.Reason != "signature_invalid" {
		t.Fatalf("dev 控制面无签名应拒绝,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
}
