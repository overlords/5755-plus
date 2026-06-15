package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestCallbackSignHMAC 钉死充值回调签名的 HMAC-SHA256 口径(04 §4 / ADR-0016):
//   - secret 作 HMAC 密钥、不拼进串(旧 MD5 口径末尾的 `key=<secret>` 已移除);
//   - 待签串 = 除 sign 外全部键按字典序 `k=v&` 逐对拼接(含最后一对的 &)。
func TestCallbackSignHMAC(t *testing.T) {
	const secret = "m5755-dev-callback-secret-v1"
	params := map[string]string{
		"account":         "s_abc123",
		"platformOrderId": "P5755123456789",
		"cpOrderId":       "cp_order_1",
		"amount":          "6.00",
		"serverName":      "星河一区", // 中文 UTF-8 字节参与签名
	}

	// 独立复算 canonical 串 = 字典序 k=v& 逐对(含末尾 &),不含 key=secret。
	canonical := "account=s_abc123&amount=6.00&cpOrderId=cp_order_1&platformOrderId=P5755123456789&serverName=星河一区&"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonical))
	want := hex.EncodeToString(mac.Sum(nil))

	got := callbackSign(params, secret)
	if got != want {
		t.Fatalf("callbackSign 与独立 HMAC-SHA256 复算不一致:\n got=%s\nwant=%s\n(secret 必须作 HMAC 密钥、不拼进串)", got, want)
	}

	// sign 字段不参与签名:带上 sign 后重算应不变。
	params["sign"] = got
	if again := callbackSign(params, secret); again != got {
		t.Fatalf("sign 字段被纳入签名计算:%s != %s", again, got)
	}

	// 正向验签通过。
	if !VerifyCallbackSign(params, secret) {
		t.Fatal("VerifyCallbackSign 对正确签名应通过")
	}

	// 篡改业务字段 → 拒绝(签名覆盖内容完整性)。
	tampered := make(map[string]string, len(params))
	for k, v := range params {
		tampered[k] = v
	}
	tampered["amount"] = "9999.00"
	if VerifyCallbackSign(tampered, secret) {
		t.Fatal("篡改 amount 后验签仍通过——签名未覆盖该字段")
	}

	// 错误密钥 → 拒绝(secret 确实作 HMAC 密钥)。
	if VerifyCallbackSign(params, "wrong-secret") {
		t.Fatal("用错误密钥验签仍通过——secret 未作 HMAC 密钥参与计算")
	}
}
