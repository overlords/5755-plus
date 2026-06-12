package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"m5755/server/internal/domain"
	"m5755/server/internal/sms"
	"m5755/server/internal/store"
)

func smsTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("未设置 DATABASE_URL,跳过短信 provider 门控测试")
	}
	st, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

// 生产口径(smsMock=false)+ 京东云就绪:真发成功,响应 providerMode=jdcloud 且【绝不含 devCode】。
func TestSmsJdcloudOmitsDevCode(t *testing.T) {
	st := smsTestStore(t)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requestId":"req-x","result":{"code":"0","status":"success","data":{"sequenceNumber":"seq-x"}}}`))
	}))
	defer fake.Close()

	cfg := sms.Config{AccessKeyID: "AKID12345", AccessKeySecret: "secretsecret", SignID: "sign-1", TemplateID: "tmpl-1", Region: "cn-north-1", Endpoint: fake.URL}
	svc := domain.NewWith(st, domain.Options{CallbackSecret: "x", SmsMock: false, SmsConfig: cfg})
	srv := httptest.NewServer(NewRouter(svc, st, time.Now, "http://127.0.0.1:0"))
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"gameId": seedGame, "loginAccount": "13800139000"})
	resp, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/sms-codes", "", body, 0, false, false)
	if resp.StatusCode != 200 || !ar.Success {
		t.Fatalf("jdcloud 发送应成功,得 status=%d reason=%s", resp.StatusCode, ar.Reason)
	}
	if ar.Data["providerMode"] != "jdcloud" {
		t.Fatalf("providerMode 应为 jdcloud,得 %v", ar.Data["providerMode"])
	}
	if _, ok := ar.Data["devCode"]; ok {
		t.Fatalf("生产 jdcloud 模式绝不能返回 devCode:data=%v", ar.Data)
	}
}

// 生产口径但京东云凭据未就绪:fail-closed(503),绝不退回 mock、绝不返回 devCode。
func TestSmsFailClosedWhenNotReady(t *testing.T) {
	st := smsTestStore(t)
	svc := domain.NewWith(st, domain.Options{CallbackSecret: "x", SmsMock: false, SmsConfig: sms.Config{}})
	srv := httptest.NewServer(NewRouter(svc, st, time.Now, "http://127.0.0.1:0"))
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"gameId": seedGame, "loginAccount": "13800139001"})
	resp, ar := doSigned(t, srv.URL, "POST", "/api/sdk/v2/sms-codes", "", body, 0, false, false)
	if resp.StatusCode == 200 || ar.Success {
		t.Fatalf("凭据未就绪应 fail-closed,得 status=%d success=%v", resp.StatusCode, ar.Success)
	}
	if _, ok := ar.Data["devCode"]; ok {
		t.Fatal("fail-closed 时绝不能返回 devCode")
	}
}
