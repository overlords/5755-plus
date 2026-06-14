package domain

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"m5755/server/internal/store"
)

// TestDispatchCallbackLogsChainWithoutSecret:回调投递打出可按订单号/CP 单号检索的日志,
// 且绝不泄漏 callbackSecret 明文(#57 业务日志 + 安全约束)。不依赖数据库。
func TestDispatchCallbackLogsChainWithoutSecret(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"msg":"success"}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	lg := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	const secret = "super-secret-callback-key-v1"
	svc := NewWith(nil, Options{CallbackSecret: secret, Logger: lg})

	o := &store.Order{
		PlatformOrderID: "P5755TEST1", CPOrderID: "cp-abc-123", Account: "ga_57550001_0025",
		Amount: "1.00", Commodity: "钻石", ServerID: "s1", ServerName: "一区",
	}
	if ok := svc.dispatchCallback(ts.URL, o); !ok {
		t.Fatalf("dispatchCallback 应在游戏端回 {code:200,msg:success} 时确认成功")
	}

	out := buf.String()
	if !strings.Contains(out, "callback_attempt") || !strings.Contains(out, "P5755TEST1") {
		t.Errorf("日志应可按 platformOrderId 检索 callback_attempt;实际:\n%s", out)
	}
	if !strings.Contains(out, "cp-abc-123") {
		t.Errorf("日志应含 cpOrderId 以便对账;实际:\n%s", out)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("日志泄漏了 callbackSecret 明文!实际:\n%s", out)
	}
}

// TestMaskAccount:account 确定性脱敏——同输入恒同输出(可检索)、中段隐藏(不漏完整账户)。
func TestMaskAccount(t *testing.T) {
	full := "ga_57550001_0025"
	got := maskAccount(full)
	if got == full {
		t.Errorf("应脱敏,不得原样返回:%s", got)
	}
	if strings.Contains(got, "57550001") {
		t.Errorf("脱敏后不得含账户中段:%s", got)
	}
	if got != maskAccount(full) {
		t.Errorf("脱敏须确定性(同输入同输出)")
	}
	if maskAccount("") != "" {
		t.Errorf("空账户应返回空")
	}
	if maskAccount("short") != "****" {
		t.Errorf("过短账户应整体脱敏为 ****,实际:%s", maskAccount("short"))
	}
}
