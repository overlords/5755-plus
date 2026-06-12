package sms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeJDCloud(t *testing.T, captured *map[string]any, capturedHeaders *http.Header, respBody string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/regions/") || !strings.HasSuffix(r.URL.Path, "/batchSend") {
			t.Errorf("意外的请求 %s %s", r.Method, r.URL.Path)
		}
		*capturedHeaders = r.Header.Clone()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		*captured = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
}

func goodConfig(endpoint string) Config {
	return Config{
		AccessKeyID:     "AKID12345",
		AccessKeySecret: "secretsecret",
		SignID:          "sign-1",
		TemplateID:      "tmpl-1",
		Region:          "cn-north-1",
		Endpoint:        endpoint,
	}
}

func TestSendJDCloudSuccess(t *testing.T) {
	var body map[string]any
	var hdr http.Header
	srv := fakeJDCloud(t, &body, &hdr, `{"requestId":"req-1","result":{"code":"0","status":"success","data":{"sequenceNumber":"seq-1"}}}`, 200)
	defer srv.Close()

	res, err := SendJDCloud(context.Background(), srv.Client(), goodConfig(srv.URL), "13800138000", "123456", time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("发送应成功: %v", err)
	}
	if res.ProviderRequestID != "req-1" || res.ProviderBizID != "seq-1" {
		t.Fatalf("解析错误: %+v", res)
	}
	// 签名头格式
	auth := hdr.Get("Authorization")
	if !strings.HasPrefix(auth, "JDCLOUD2-HMAC-SHA256 Credential=AKID12345/") {
		t.Fatalf("Authorization 头格式错: %q", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=") || !strings.Contains(auth, "Signature=") {
		t.Fatalf("Authorization 缺字段: %q", auth)
	}
	if hdr.Get("X-Jdcloud-Date") == "" || hdr.Get("X-Jdcloud-Nonce") == "" {
		t.Fatal("缺 x-jdcloud-date/nonce 头")
	}
	// body 形态:params=[code]、phoneList=[phone]、signId、templateId
	if got := body["signId"]; got != "sign-1" {
		t.Fatalf("signId=%v", got)
	}
	if got := body["templateId"]; got != "tmpl-1" {
		t.Fatalf("templateId=%v", got)
	}
	params, _ := body["params"].([]any)
	if len(params) != 1 || params[0] != "123456" {
		t.Fatalf("params=%v(验证码须随 params 下发)", body["params"])
	}
	phones, _ := body["phoneList"].([]any)
	if len(phones) != 1 || phones[0] != "13800138000" {
		t.Fatalf("phoneList=%v", body["phoneList"])
	}
}

func TestSendJDCloudProviderFailure(t *testing.T) {
	var body map[string]any
	var hdr http.Header
	srv := fakeJDCloud(t, &body, &hdr, `{"result":{"code":"PARAM_ERROR","status":"false","message":"模板不存在"}}`, 200)
	defer srv.Close()
	_, err := SendJDCloud(context.Background(), srv.Client(), goodConfig(srv.URL), "13800138000", "123456", time.Unix(1700000000, 0))
	if err == nil {
		t.Fatal("provider 返回非接受态时应报错,不得静默成功")
	}
}

func TestConfigValidate(t *testing.T) {
	if f := goodConfig("https://sms.jdcloud-api.com").Validate(); len(f) != 0 {
		t.Fatalf("合法配置应就绪,得 %v", f)
	}
	empty := Config{}
	f := empty.Validate()
	if len(f) == 0 {
		t.Fatal("空配置应判定未就绪")
	}
	// 非法 region
	bad := goodConfig("https://x")
	bad.Region = "north1"
	if f := bad.Validate(); len(f) == 0 {
		t.Fatal("非法 region 应被拒")
	}
}
